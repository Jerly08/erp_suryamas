'use client';

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Badge,
  Box,
  Button,
  Divider,
  Flex,
  FormControl,
  FormHelperText,
  FormLabel,
  HStack,
  Icon,
  Input,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Select,
  SimpleGrid,
  Spinner,
  Text,
  Textarea,
  Tooltip,
  useColorModeValue,
  useDisclosure,
  useToast,
  VStack,
} from '@chakra-ui/react';
import {
  FiCheckCircle,
  FiClock,
  FiEdit3,
  FiFileText,
  FiPaperclip,
  FiRefreshCw,
  FiXCircle,
} from 'react-icons/fi';
import { useAuth } from '@/contexts/AuthContext';
import SearchableSelect from '@/components/common/SearchableSelect';
import accountService from '@/services/accountService';
import type { AccountCatalogItem } from '@/types/account';
import postedTransactionCorrectionService, {
  type CreatePostedTransactionCorrectionRequest,
  type PostedTransactionCorrection,
  type PostedTransactionCorrectionResolution,
  type PostedTransactionCorrectionStatus,
  type PostedTransactionCorrectionType,
} from '@/services/postedTransactionCorrectionService';
import {
  isAccountingRole,
  isFinanceManagerRole,
  isFinanceStaffRole,
  normalizeRole,
} from '@/utils/roles';

type CorrectionTarget = {
  purchaseId?: number;
  paymentId?: number;
  code?: string;
};

type ActionModalState =
  | { type: 'accounting'; correction: PostedTransactionCorrection }
  | { type: 'finance'; correction: PostedTransactionCorrection; approve: boolean }
  | { type: 'complete'; correction: PostedTransactionCorrection }
  | null;

const correctionTypeOptions: Array<{ value: PostedTransactionCorrectionType; label: string }> = [
  { value: 'MISSING_PAYMENT_PROOF', label: 'Bukti pembayaran belum atau perlu dilengkapi' },
  { value: 'PAYMENT_DATE', label: 'Tanggal pembayaran tidak sesuai' },
  { value: 'PAYMENT_AMOUNT', label: 'Nominal pembayaran tidak sesuai' },
  { value: 'SOURCE_BANK', label: 'Rekening sumber tidak sesuai' },
  { value: 'DESTINATION_ACCOUNT', label: 'Rekening tujuan atau penerima tidak sesuai' },
  { value: 'PURCHASE_DATE', label: 'Tanggal pembelian tidak sesuai' },
  { value: 'OTHER', label: 'Koreksi lain' },
];

const resolutionOptions: Array<{ value: PostedTransactionCorrectionResolution; label: string }> = [
  { value: 'SUPPLEMENTAL_PROOF', label: 'Lengkapi bukti tanpa mengubah jurnal' },
  { value: 'JOURNAL_RECLASSIFICATION', label: 'Jurnal reklasifikasi' },
  { value: 'REVERSE_AND_RECREATE', label: 'Reversal dan buat ulang transaksi' },
  { value: 'BANK_RECONCILIATION', label: 'Rekonsiliasi bank' },
  { value: 'MANUAL_ADJUSTMENT', label: 'Jurnal penyesuaian terkontrol' },
];

const statusPresentation: Record<PostedTransactionCorrectionStatus, { label: string; colorScheme: string }> = {
  PENDING_ACCOUNTING: { label: 'Menunggu Accounting', colorScheme: 'blue' },
  PENDING_FINANCE_MANAGER: { label: 'Menunggu Finance Manager', colorScheme: 'purple' },
  APPROVED_FOR_EXECUTION: { label: 'Disetujui untuk dieksekusi', colorScheme: 'orange' },
  REJECTED: { label: 'Ditolak', colorScheme: 'red' },
  COMPLETED: { label: 'Selesai', colorScheme: 'green' },
  CANCELLED: { label: 'Dibatalkan', colorScheme: 'gray' },
};

const formatCurrency = (value?: number | null) =>
  new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    maximumFractionDigits: 2,
  }).format(Number(value || 0));

const formatDateTime = (value?: string | null) => {
  if (!value) return '-';
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) return '-';
  return parsed.toLocaleString('id-ID', {
    day: '2-digit',
    month: '2-digit',
    year: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  });
};

const formatFileSize = (value?: number) => {
  const bytes = Number(value || 0);
  if (bytes < 1024) return `${bytes} B`;
  if (bytes < 1024 * 1024) return `${(bytes / 1024).toFixed(1)} KB`;
  return `${(bytes / (1024 * 1024)).toFixed(1)} MB`;
};

const actorName = (actor?: { username?: string; first_name?: string; last_name?: string; email?: string }) => {
  const fullName = [actor?.first_name, actor?.last_name].filter(Boolean).join(' ').trim();
  return fullName || actor?.username || actor?.email || 'Sistem';
};

const correctionTypeLabel = (type: string) =>
  correctionTypeOptions.find((option) => option.value === type)?.label || type.replace(/_/g, ' ');

const getErrorDescription = (error: unknown, fallback = 'Coba lagi beberapa saat lagi.') => {
  if (typeof error === 'object' && error !== null) {
    const apiError = error as {
      message?: unknown;
      response?: { data?: { error?: unknown; message?: unknown } };
    };
    const responseMessage = apiError.response?.data?.error || apiError.response?.data?.message;
    if (typeof responseMessage === 'string' && responseMessage.trim()) return responseMessage;
    if (typeof apiError.message === 'string' && apiError.message.trim()) return apiError.message;
  }
  return fallback;
};

interface WorkflowContentProps {
  target?: CorrectionTarget;
  showCreate?: boolean;
  onChanged?: () => void;
}

const WorkflowContent: React.FC<WorkflowContentProps> = ({
  target,
  showCreate = false,
  onChanged,
}) => {
  const { token, user } = useAuth();
  const toast = useToast();
  const role = normalizeRole(user?.role);
  const currentUserID = Number(user?.id);
  const canAccessWorkflow = isFinanceStaffRole(role) || isAccountingRole(role) || isFinanceManagerRole(role) || role === 'admin';
  const canRequest = Boolean(target && (isFinanceStaffRole(role) || isFinanceManagerRole(role) || role === 'admin'));
  const canReviewAccounting = isAccountingRole(role) || role === 'admin';
  const canReviewFinance = isFinanceManagerRole(role) || role === 'admin';
  const canComplete = isAccountingRole(role) || role === 'admin';

  const [corrections, setCorrections] = useState<PostedTransactionCorrection[]>([]);
  const [loading, setLoading] = useState(false);
  const [saving, setSaving] = useState(false);
  const [uploadingCorrectionId, setUploadingCorrectionId] = useState<number | null>(null);
  const [actionModal, setActionModal] = useState<ActionModalState>(null);
  const [accounts, setAccounts] = useState<AccountCatalogItem[]>([]);
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [reviewResolution, setReviewResolution] = useState<PostedTransactionCorrectionResolution>('JOURNAL_RECLASSIFICATION');
  const [reviewNotes, setReviewNotes] = useState('');
  const [proposedAccountId, setProposedAccountId] = useState<number | undefined>();
  const [decisionNotes, setDecisionNotes] = useState('');
  const [completionNotes, setCompletionNotes] = useState('');
  const [completionJournalId, setCompletionJournalId] = useState('');
  const [replacementPaymentId, setReplacementPaymentId] = useState('');
  const uploadInputRef = useRef<HTMLInputElement>(null);
  const createDisclosure = useDisclosure();

  const purchaseID = target?.purchaseId;
  const paymentID = target?.paymentId;
  const hasTarget = Boolean(target);

  const loadCorrections = useCallback(async () => {
    if (!token || !canAccessWorkflow) return;
    setLoading(true);
    try {
      const response = await postedTransactionCorrectionService.list(token, {
        purchase_id: purchaseID,
        payment_id: paymentID,
        limit: hasTarget ? 50 : 100,
      });
      setCorrections(response.data || []);
    } catch (error: unknown) {
      console.error('Failed to load posted transaction corrections', error);
      toast({
        title: 'Koreksi transaksi tidak dapat dimuat',
        description: getErrorDescription(error, 'Coba muat ulang beberapa saat lagi.'),
        status: 'error',
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  }, [canAccessWorkflow, hasTarget, paymentID, purchaseID, toast, token]);

  useEffect(() => {
    void loadCorrections();
  }, [loadCorrections]);

  useEffect(() => {
    if (!actionModal || actionModal.type !== 'accounting' || !token) return;
    setLoadingAccounts(true);
    accountService
      .getAccountCatalog(token)
      .then((rows) => {
        setAccounts(
          rows.filter((account) => {
            const active = account.active ?? account.is_active ?? true;
            return active && !account.is_header;
          }),
        );
      })
      .catch((error) => {
        console.error('Failed to load account catalog for correction', error);
        toast({
          title: 'Daftar COA tidak dapat dimuat',
          description: 'Anda masih dapat menyimpan review tanpa memilih COA usulan.',
          status: 'warning',
          duration: 5000,
        });
      })
      .finally(() => setLoadingAccounts(false));
  }, [actionModal, toast, token]);

  const refreshAfterChange = async () => {
    await loadCorrections();
    onChanged?.();
  };

  const openAccountingReview = (correction: PostedTransactionCorrection) => {
    setReviewResolution(correction.accounting_resolution || 'JOURNAL_RECLASSIFICATION');
    setReviewNotes(correction.accounting_notes || '');
    setProposedAccountId(correction.proposed_account_id || undefined);
    setActionModal({ type: 'accounting', correction });
  };

  const openFinanceDecision = (correction: PostedTransactionCorrection, approve: boolean) => {
    setDecisionNotes(correction.finance_manager_decision_notes || '');
    setActionModal({ type: 'finance', correction, approve });
  };

  const openCompletion = (correction: PostedTransactionCorrection) => {
    setCompletionNotes(correction.completion_notes || '');
    setCompletionJournalId(correction.completion_journal_entry_id ? String(correction.completion_journal_entry_id) : '');
    setReplacementPaymentId(correction.replacement_payment_id ? String(correction.replacement_payment_id) : '');
    setActionModal({ type: 'complete', correction });
  };

  const handleAccountingReview = async () => {
    if (!actionModal || actionModal.type !== 'accounting' || !token) return;
    if (!reviewNotes.trim()) {
      toast({ title: 'Catatan Accounting wajib diisi', status: 'warning', duration: 3500 });
      return;
    }
    setSaving(true);
    try {
      await postedTransactionCorrectionService.reviewAccounting(token, actionModal.correction.id, {
        resolution: reviewResolution,
        notes: reviewNotes.trim(),
        proposed_account_id: proposedAccountId,
      });
      toast({ title: 'Review Accounting disimpan', description: 'Koreksi diteruskan ke Finance Manager.', status: 'success', duration: 3500 });
      setActionModal(null);
      await refreshAfterChange();
    } catch (error: unknown) {
      toast({ title: 'Review Accounting gagal disimpan', description: getErrorDescription(error), status: 'error', duration: 5000 });
    } finally {
      setSaving(false);
    }
  };

  const handleFinanceDecision = async () => {
    if (!actionModal || actionModal.type !== 'finance' || !token) return;
    if (!actionModal.approve && !decisionNotes.trim()) {
      toast({ title: 'Alasan penolakan wajib diisi', status: 'warning', duration: 3500 });
      return;
    }
    setSaving(true);
    try {
      await postedTransactionCorrectionService.decideFinanceManager(token, actionModal.correction.id, {
        approve: actionModal.approve,
        notes: decisionNotes.trim(),
      });
      toast({
        title: actionModal.approve ? 'Koreksi disetujui untuk dieksekusi' : 'Koreksi ditolak',
        status: actionModal.approve ? 'success' : 'info',
        duration: 3500,
      });
      setActionModal(null);
      await refreshAfterChange();
    } catch (error: unknown) {
      toast({ title: 'Keputusan Finance Manager gagal disimpan', description: getErrorDescription(error), status: 'error', duration: 5000 });
    } finally {
      setSaving(false);
    }
  };

  const handleComplete = async () => {
    if (!actionModal || actionModal.type !== 'complete' || !token) return;
    if (!completionNotes.trim()) {
      toast({ title: 'Catatan penyelesaian wajib diisi', status: 'warning', duration: 3500 });
      return;
    }
    const parsedJournalId = completionJournalId.trim() ? Number(completionJournalId) : undefined;
    const parsedPaymentId = replacementPaymentId.trim() ? Number(replacementPaymentId) : undefined;
    if ((parsedJournalId !== undefined && (!Number.isInteger(parsedJournalId) || parsedJournalId <= 0)) ||
        (parsedPaymentId !== undefined && (!Number.isInteger(parsedPaymentId) || parsedPaymentId <= 0))) {
      toast({ title: 'ID jurnal atau pembayaran pengganti tidak valid', status: 'warning', duration: 3500 });
      return;
    }
    setSaving(true);
    try {
      await postedTransactionCorrectionService.complete(token, actionModal.correction.id, {
        notes: completionNotes.trim(),
        completion_journal_entry_id: parsedJournalId,
        replacement_payment_id: parsedPaymentId,
      });
      toast({ title: 'Koreksi ditandai selesai', description: 'Transaksi asal tetap tersimpan sebagai riwayat audit.', status: 'success', duration: 3500 });
      setActionModal(null);
      await refreshAfterChange();
    } catch (error: unknown) {
      toast({ title: 'Penyelesaian koreksi gagal disimpan', description: getErrorDescription(error), status: 'error', duration: 5000 });
    } finally {
      setSaving(false);
    }
  };

  const handleUploadDocument = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    const correctionId = uploadingCorrectionId;
    event.target.value = '';
    if (!file || !correctionId || !token) return;

    setSaving(true);
    try {
      await postedTransactionCorrectionService.uploadDocument(token, correctionId, file);
      toast({ title: 'Dokumen koreksi ditambahkan', status: 'success', duration: 3500 });
      await refreshAfterChange();
    } catch (error: unknown) {
      toast({ title: 'Dokumen koreksi gagal diunggah', description: getErrorDescription(error), status: 'error', duration: 5000 });
    } finally {
      setSaving(false);
      setUploadingCorrectionId(null);
    }
  };

  const queueTitle = target ? `Riwayat koreksi ${target.code || 'transaksi'}` : 'Antrean koreksi transaksi terposting';

  if (!canAccessWorkflow) return null;

  return (
    <VStack align="stretch" spacing={4}>
      <Flex justify="space-between" align={{ base: 'stretch', md: 'center' }} direction={{ base: 'column', md: 'row' }} gap={3}>
        <Box>
          <HStack spacing={2}>
            <Icon as={FiEdit3} color="orange.500" />
            <Text fontWeight="bold">{queueTitle}</Text>
          </HStack>
          <Text mt={1} fontSize="sm" color="gray.500">
            Transaksi asal tidak diubah. Setiap koreksi ditelaah Accounting dan memerlukan keputusan Finance Manager.
          </Text>
        </Box>
        <HStack alignSelf={{ base: 'flex-end', md: 'auto' }}>
          <Tooltip label="Muat ulang riwayat koreksi">
            <Button aria-label="Muat ulang koreksi" size="sm" variant="outline" leftIcon={<FiRefreshCw />} onClick={() => void loadCorrections()} isLoading={loading}>
              Muat ulang
            </Button>
          </Tooltip>
          {showCreate && canRequest && (
            <Button size="sm" colorScheme="orange" leftIcon={<FiEdit3 />} onClick={createDisclosure.onOpen}>
              Ajukan Koreksi
            </Button>
          )}
        </HStack>
      </Flex>

      {loading && corrections.length === 0 ? (
        <Flex py={6} justify="center"><Spinner /></Flex>
      ) : corrections.length === 0 ? (
        <Alert status="info" variant="left-accent">
          <AlertIcon />
          <AlertDescription>
            Belum ada pengajuan koreksi untuk {target ? 'transaksi ini' : 'transaksi terposting'}.
          </AlertDescription>
        </Alert>
      ) : (
        <VStack align="stretch" spacing={3}>
          {corrections.map((correction) => {
            const presentation = statusPresentation[correction.status] || { label: correction.status, colorScheme: 'gray' };
            const canAttach = correction.status !== 'CANCELLED' && (
              (Number.isInteger(currentUserID) && correction.requested_by === currentUserID) || canReviewAccounting || canReviewFinance
            );
            return (
              <Box key={correction.id} borderWidth="1px" borderRadius="md" p={4}>
                <Flex justify="space-between" align={{ base: 'flex-start', md: 'center' }} direction={{ base: 'column', md: 'row' }} gap={2}>
                  <Box minW={0}>
                    <HStack spacing={2} wrap="wrap">
                      <Text fontWeight="bold">{correction.target_code}</Text>
                      <Badge colorScheme={presentation.colorScheme}>{presentation.label}</Badge>
                      <Badge variant="subtle">{correctionTypeLabel(correction.correction_type)}</Badge>
                    </HStack>
                    <Text mt={1} fontSize="sm" color="gray.500">
                      Diajukan {formatDateTime(correction.requested_at)} oleh {actorName(correction.requester)}
                    </Text>
                  </Box>
                  <HStack flexWrap="wrap">
                    {canAttach && (
                      <Button size="xs" variant="outline" leftIcon={<FiPaperclip />} onClick={() => {
                        setUploadingCorrectionId(correction.id);
                        uploadInputRef.current?.click();
                      }} isLoading={saving && uploadingCorrectionId === correction.id}>
                        Tambah Dokumen
                      </Button>
                    )}
                    {canReviewAccounting && correction.status === 'PENDING_ACCOUNTING' && (
                      <Button size="xs" colorScheme="blue" leftIcon={<FiClock />} onClick={() => openAccountingReview(correction)}>
                        Review Accounting
                      </Button>
                    )}
                    {canReviewFinance && correction.status === 'PENDING_FINANCE_MANAGER' && (
                      <>
                        <Button size="xs" colorScheme="green" leftIcon={<FiCheckCircle />} onClick={() => openFinanceDecision(correction, true)}>
                          Setujui
                        </Button>
                        <Button size="xs" colorScheme="red" variant="outline" leftIcon={<FiXCircle />} onClick={() => openFinanceDecision(correction, false)}>
                          Tolak
                        </Button>
                      </>
                    )}
                    {canComplete && correction.status === 'APPROVED_FOR_EXECUTION' && (
                      <Button size="xs" colorScheme="green" leftIcon={<FiCheckCircle />} onClick={() => openCompletion(correction)}>
                        Tandai Selesai
                      </Button>
                    )}
                  </HStack>
                </Flex>

                <Text mt={3} fontSize="sm" whiteSpace="pre-wrap">{correction.reason}</Text>
                {(correction.requested_date || correction.requested_amount || correction.requested_cash_bank_reference || correction.requested_destination_reference || correction.requested_reference) && (
                  <SimpleGrid mt={3} columns={{ base: 1, md: 2 }} spacing={2} fontSize="sm">
                    {correction.requested_date && <Text><Text as="span" color="gray.500">Tanggal aktual: </Text>{new Date(correction.requested_date).toLocaleDateString('id-ID')}</Text>}
                    {correction.requested_amount !== undefined && correction.requested_amount !== null && <Text><Text as="span" color="gray.500">Nominal aktual: </Text>{formatCurrency(correction.requested_amount)}</Text>}
                    {correction.requested_cash_bank_reference && <Text><Text as="span" color="gray.500">Rekening sumber aktual: </Text>{correction.requested_cash_bank_reference}</Text>}
                    {correction.requested_destination_reference && <Text><Text as="span" color="gray.500">Tujuan aktual: </Text>{correction.requested_destination_reference}</Text>}
                    {correction.requested_reference && <Text><Text as="span" color="gray.500">Referensi aktual: </Text>{correction.requested_reference}</Text>}
                  </SimpleGrid>
                )}

                {correction.accounting_resolution && (
                  <Box mt={3} borderTopWidth="1px" pt={3} fontSize="sm">
                    <Text fontWeight="semibold">Rencana Accounting</Text>
                    <Text>{resolutionOptions.find((option) => option.value === correction.accounting_resolution)?.label || correction.accounting_resolution}</Text>
                    {correction.proposed_account && <Text color="gray.500">COA usulan: {correction.proposed_account.code} - {correction.proposed_account.name}</Text>}
                    {correction.accounting_notes && <Text mt={1} whiteSpace="pre-wrap">{correction.accounting_notes}</Text>}
                  </Box>
                )}

                {correction.finance_manager_decision_notes && (
                  <Text mt={3} fontSize="sm" whiteSpace="pre-wrap"><Text as="span" fontWeight="semibold">Catatan Finance Manager: </Text>{correction.finance_manager_decision_notes}</Text>
                )}
                {correction.completion_notes && (
                  <Text mt={3} fontSize="sm" whiteSpace="pre-wrap"><Text as="span" fontWeight="semibold">Penyelesaian: </Text>{correction.completion_notes}</Text>
                )}

                {(correction.documents?.length || correction.events?.length) ? <Divider mt={3} /> : null}
                {correction.documents && correction.documents.length > 0 && (
                  <VStack mt={3} align="stretch" spacing={1}>
                    {correction.documents.map((document) => (
                      <Flex key={document.id} justify="space-between" gap={3} align="center" fontSize="sm">
                        <HStack minW={0} spacing={2}>
                          <Icon as={FiFileText} color="blue.500" flexShrink={0} />
                          <Text noOfLines={1}>{document.file_name}</Text>
                          <Text color="gray.500" flexShrink={0}>{formatFileSize(document.file_size)}</Text>
                        </HStack>
                        <Button size="xs" variant="outline" onClick={() => {
                          if (token) void postedTransactionCorrectionService.downloadDocument(token, document).catch((error) => {
                            toast({ title: 'Dokumen tidak dapat diunduh', description: error?.response?.data?.error || error?.message, status: 'error', duration: 5000 });
                          });
                        }}>
                          Buka
                        </Button>
                      </Flex>
                    ))}
                  </VStack>
                )}
                {correction.events && correction.events.length > 0 && (
                  <Text mt={3} fontSize="xs" color="gray.500">
                    Aktivitas terakhir: {correction.events[correction.events.length - 1].event_type.replace(/_/g, ' ')} oleh {actorName(correction.events[correction.events.length - 1].actor)} pada {formatDateTime(correction.events[correction.events.length - 1].created_at)}
                  </Text>
                )}
              </Box>
            );
          })}
        </VStack>
      )}

      <Input ref={uploadInputRef} display="none" type="file" accept=".pdf,.doc,.docx,.xls,.xlsx,.csv,.txt,.jpg,.jpeg,.png,.webp" onChange={handleUploadDocument} />

      <CreateCorrectionModal
        isOpen={createDisclosure.isOpen}
        onClose={createDisclosure.onClose}
        target={target}
        token={token || undefined}
        onSuccess={async () => {
          createDisclosure.onClose();
          await refreshAfterChange();
        }}
      />

      <Modal isOpen={Boolean(actionModal)} onClose={() => !saving && setActionModal(null)} size="lg" isCentered>
        <ModalOverlay />
        <ModalContent maxH="calc(100vh - 32px)" display="flex" flexDirection="column" overflow="hidden">
          <ModalHeader>
            {actionModal?.type === 'accounting' && 'Review Koreksi oleh Accounting'}
            {actionModal?.type === 'finance' && (actionModal.approve ? 'Setujui Koreksi Transaksi' : 'Tolak Koreksi Transaksi')}
            {actionModal?.type === 'complete' && 'Catat Penyelesaian Koreksi'}
          </ModalHeader>
          <ModalCloseButton isDisabled={saving} />
          <ModalBody flex="1" minH="0" overflowY="auto">
            {actionModal?.type === 'accounting' && (
              <VStack align="stretch" spacing={4}>
                <Alert status="info" variant="left-accent"><AlertIcon /><AlertDescription>Accounting menentukan perlakuan koreksi. Tidak ada jurnal yang dibuat otomatis pada tahap ini.</AlertDescription></Alert>
                <FormControl isRequired>
                  <FormLabel>Rencana penyelesaian</FormLabel>
                  <Select value={reviewResolution} onChange={(event) => setReviewResolution(event.target.value as PostedTransactionCorrectionResolution)}>
                    {resolutionOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                  </Select>
                </FormControl>
                <FormControl>
                  <FormLabel>COA usulan (opsional)</FormLabel>
                  <SearchableSelect
                    options={accounts.map((account) => ({ id: account.id, code: account.code, name: account.name, active: account.active ?? account.is_active }))}
                    value={proposedAccountId}
                      onChange={(value) => {
                        const accountID = Number(value);
                        setProposedAccountId(Number.isInteger(accountID) && accountID > 0 ? accountID : undefined);
                      }}
                    placeholder={loadingAccounts ? 'Memuat daftar COA...' : 'Cari kode atau nama COA'}
                    isLoading={loadingAccounts}
                    allowClear
                  />
                  <FormHelperText>Isi bila Accounting sudah mengetahui akun yang akan digunakan saat eksekusi koreksi.</FormHelperText>
                </FormControl>
                <FormControl isRequired>
                  <FormLabel>Catatan Accounting</FormLabel>
                  <Textarea value={reviewNotes} onChange={(event) => setReviewNotes(event.target.value)} rows={5} placeholder="Jelaskan perlakuan koreksi, dasar bukti, dan langkah eksekusinya." />
                </FormControl>
              </VStack>
            )}
            {actionModal?.type === 'finance' && (
              <VStack align="stretch" spacing={4}>
                <Alert status={actionModal.approve ? 'success' : 'warning'} variant="left-accent"><AlertIcon /><AlertDescription>{actionModal.approve ? 'Persetujuan hanya membuka koreksi untuk dieksekusi oleh Accounting. Transaksi dan jurnal asal tetap tidak diubah.' : 'Penolakan akan menutup pengajuan ini tanpa mengubah transaksi asal.'}</AlertDescription></Alert>
                <FormControl isRequired={!actionModal.approve}>
                  <FormLabel>{actionModal.approve ? 'Catatan persetujuan (opsional)' : 'Alasan penolakan'}</FormLabel>
                  <Textarea value={decisionNotes} onChange={(event) => setDecisionNotes(event.target.value)} rows={5} placeholder={actionModal.approve ? 'Catatan atau syarat eksekusi bila diperlukan.' : 'Jelaskan alasan pengajuan belum dapat disetujui.'} />
                </FormControl>
              </VStack>
            )}
            {actionModal?.type === 'complete' && (
              <VStack align="stretch" spacing={4}>
                <Alert status="info" variant="left-accent"><AlertIcon /><AlertDescription>Isi referensi eksekusi bila sudah ada. Menandai selesai tidak mengubah jurnal asal secara otomatis.</AlertDescription></Alert>
                <FormControl isRequired>
                  <FormLabel>Catatan penyelesaian</FormLabel>
                  <Textarea value={completionNotes} onChange={(event) => setCompletionNotes(event.target.value)} rows={5} placeholder="Jelaskan tindakan yang sudah dilakukan dan bukti yang dipakai." />
                </FormControl>
                <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                  <FormControl>
                    <FormLabel>ID jurnal koreksi (opsional)</FormLabel>
                    <Input inputMode="numeric" value={completionJournalId} onChange={(event) => setCompletionJournalId(event.target.value)} placeholder="Contoh: 12345" />
                  </FormControl>
                  <FormControl>
                    <FormLabel>ID pembayaran pengganti (opsional)</FormLabel>
                    <Input inputMode="numeric" value={replacementPaymentId} onChange={(event) => setReplacementPaymentId(event.target.value)} placeholder="Contoh: 12345" />
                  </FormControl>
                </SimpleGrid>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={() => setActionModal(null)} isDisabled={saving}>Batal</Button>
            {actionModal?.type === 'accounting' && <Button colorScheme="blue" onClick={() => void handleAccountingReview()} isLoading={saving}>Kirim ke Finance Manager</Button>}
            {actionModal?.type === 'finance' && <Button colorScheme={actionModal.approve ? 'green' : 'red'} onClick={() => void handleFinanceDecision()} isLoading={saving}>{actionModal.approve ? 'Setujui Koreksi' : 'Tolak Koreksi'}</Button>}
            {actionModal?.type === 'complete' && <Button colorScheme="green" onClick={() => void handleComplete()} isLoading={saving}>Tandai Selesai</Button>}
          </ModalFooter>
        </ModalContent>
      </Modal>
    </VStack>
  );
};

interface CreateCorrectionModalProps {
  isOpen: boolean;
  onClose: () => void;
  target?: CorrectionTarget;
  token?: string;
  onSuccess: () => Promise<void>;
}

const CreateCorrectionModal: React.FC<CreateCorrectionModalProps> = ({ isOpen, onClose, target, token, onSuccess }) => {
  const toast = useToast();
  const [saving, setSaving] = useState(false);
  const [correctionType, setCorrectionType] = useState<PostedTransactionCorrectionType>('MISSING_PAYMENT_PROOF');
  const [reason, setReason] = useState('');
  const [requestedDate, setRequestedDate] = useState('');
  const [requestedAmount, setRequestedAmount] = useState('');
  const [sourceBank, setSourceBank] = useState('');
  const [destination, setDestination] = useState('');
  const [reference, setReference] = useState('');
  const [document, setDocument] = useState<File | null>(null);

  useEffect(() => {
    if (!isOpen) return;
    setCorrectionType('MISSING_PAYMENT_PROOF');
    setReason('');
    setRequestedDate('');
    setRequestedAmount('');
    setSourceBank('');
    setDestination('');
    setReference('');
    setDocument(null);
  }, [isOpen]);

  const wantsDate = correctionType === 'PAYMENT_DATE' || correctionType === 'PURCHASE_DATE';
  const wantsAmount = correctionType === 'PAYMENT_AMOUNT';
  const wantsSource = correctionType === 'SOURCE_BANK';
  const wantsDestination = correctionType === 'DESTINATION_ACCOUNT';

  const handleSubmit = async () => {
    if (!target || !token) return;
    if (!reason.trim()) {
      toast({ title: 'Alasan koreksi wajib diisi', status: 'warning', duration: 3500 });
      return;
    }
    if (wantsAmount && (!requestedAmount || Number(requestedAmount) < 0)) {
      toast({ title: 'Nominal aktual wajib diisi dengan benar', status: 'warning', duration: 3500 });
      return;
    }
    setSaving(true);
    try {
      const payload: CreatePostedTransactionCorrectionRequest = {
        purchase_id: target.purchaseId,
        payment_id: target.paymentId,
        correction_type: correctionType,
        reason: reason.trim(),
        requested_date: requestedDate || undefined,
        requested_amount: requestedAmount ? Number(requestedAmount) : undefined,
        requested_cash_bank_reference: sourceBank.trim() || undefined,
        requested_destination_reference: destination.trim() || undefined,
        requested_reference: reference.trim() || undefined,
      };
      const correction = await postedTransactionCorrectionService.create(token, payload);
      if (document) {
        await postedTransactionCorrectionService.uploadDocument(token, correction.id, document);
      }
      toast({ title: 'Pengajuan koreksi dikirim', description: 'Pengajuan sekarang menunggu review Accounting.', status: 'success', duration: 4000 });
      await onSuccess();
    } catch (error: unknown) {
      toast({ title: 'Pengajuan koreksi gagal dikirim', description: getErrorDescription(error, 'Coba periksa data lalu ulangi.'), status: 'error', duration: 6000 });
    } finally {
      setSaving(false);
    }
  };

  return (
    <Modal isOpen={isOpen} onClose={() => !saving && onClose()} size="lg" isCentered>
      <ModalOverlay />
      <ModalContent maxH="calc(100vh - 32px)" display="flex" flexDirection="column" overflow="hidden">
        <ModalHeader>Ajukan Koreksi Transaksi Terposting</ModalHeader>
        <ModalCloseButton isDisabled={saving} />
        <ModalBody flex="1" minH="0" overflowY="auto">
          <VStack align="stretch" spacing={4}>
            <Alert status="warning" variant="left-accent"><AlertIcon /><AlertDescription>Pengajuan ini tidak mengubah transaksi atau jurnal asal. Accounting akan menentukan perlakuan, lalu Finance Manager memberi keputusan.</AlertDescription></Alert>
            <Box borderWidth="1px" borderRadius="md" p={3}>
              <Text fontWeight="semibold">Transaksi asal</Text>
              <Text fontSize="sm" color="gray.500">{target?.code || '-'}</Text>
            </Box>
            <FormControl isRequired>
              <FormLabel>Jenis koreksi</FormLabel>
              <Select value={correctionType} onChange={(event) => setCorrectionType(event.target.value as PostedTransactionCorrectionType)}>
                {correctionTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
              </Select>
            </FormControl>
            {wantsDate && <FormControl isRequired><FormLabel>Tanggal aktual</FormLabel><Input type="date" value={requestedDate} onChange={(event) => setRequestedDate(event.target.value)} /></FormControl>}
            {wantsAmount && <FormControl isRequired><FormLabel>Nominal aktual</FormLabel><Input type="number" min="0" inputMode="decimal" value={requestedAmount} onChange={(event) => setRequestedAmount(event.target.value)} placeholder="Contoh: 17670000" /></FormControl>}
            {wantsSource && <FormControl><FormLabel>Rekening sumber aktual menurut bukti bank</FormLabel><Input value={sourceBank} onChange={(event) => setSourceBank(event.target.value)} placeholder="Nomor rekening atau nama bank aktual" /></FormControl>}
            {wantsDestination && <FormControl><FormLabel>Penerima atau rekening tujuan aktual</FormLabel><Input value={destination} onChange={(event) => setDestination(event.target.value)} placeholder="Nama penerima atau nomor rekening aktual" /></FormControl>}
            <FormControl><FormLabel>Referensi aktual (opsional)</FormLabel><Input value={reference} onChange={(event) => setReference(event.target.value)} placeholder="Nomor Kopra, invoice, atau referensi lain" /></FormControl>
            <FormControl isRequired>
              <FormLabel>Alasan dan kronologi</FormLabel>
              <Textarea value={reason} onChange={(event) => setReason(event.target.value)} rows={5} placeholder="Jelaskan data yang salah, data aktual, dan dasar buktinya." />
            </FormControl>
            <FormControl>
              <FormLabel>Dokumen pendukung (opsional)</FormLabel>
              <Input type="file" accept=".pdf,.doc,.docx,.xls,.xlsx,.csv,.txt,.jpg,.jpeg,.png,.webp" p={1} onChange={(event) => setDocument(event.target.files?.[0] || null)} />
              <FormHelperText>Dokumen ditambahkan sebagai lampiran baru dan tidak mengganti bukti asli.</FormHelperText>
            </FormControl>
          </VStack>
        </ModalBody>
        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={onClose} isDisabled={saving}>Batal</Button>
          <Button colorScheme="orange" leftIcon={<FiEdit3 />} onClick={() => void handleSubmit()} isLoading={saving}>Kirim ke Accounting</Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export const PostedTransactionCorrectionsPanel: React.FC<{
  target: CorrectionTarget;
  onChanged?: () => void;
}> = ({ target, onChanged }) => (
  <WorkflowContent target={target} showCreate onChanged={onChanged} />
);

export const PostedTransactionCorrectionsQueueModal: React.FC<{
  isOpen: boolean;
  onClose: () => void;
}> = ({ isOpen, onClose }) => {
  const modalBg = useColorModeValue('white', 'gray.800');
  return (
    <Modal isOpen={isOpen} onClose={onClose} size="6xl">
      <ModalOverlay />
      <ModalContent bg={modalBg} maxH="calc(100vh - 32px)" display="flex" flexDirection="column" overflow="hidden">
        <ModalHeader>Koreksi Transaksi Terposting</ModalHeader>
        <ModalCloseButton />
        <ModalBody flex="1" minH="0" overflowY="auto" pb={6}>
          <WorkflowContent />
        </ModalBody>
      </ModalContent>
    </Modal>
  );
};
