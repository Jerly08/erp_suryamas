'use client';

import React, { useCallback, useEffect, useRef, useState } from 'react';
import {
  Alert,
  AlertDescription,
  AlertIcon,
  Badge,
  Box,
  Button,
  Checkbox,
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
  type COAReclassificationAccountCandidate,
  type CreatePostedTransactionCorrectionRequest,
  type LegacySalesCreditPaymentCandidate,
  type PostedTransactionCorrection,
  type PurchaseCOAReclassificationJournalLineCandidate,
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
  saleId?: number;
  code?: string;
  allowCreate?: boolean;
};

type ActionModalState =
  | { type: 'accounting'; correction: PostedTransactionCorrection }
  | { type: 'finance'; correction: PostedTransactionCorrection; approve: boolean }
  | { type: 'complete'; correction: PostedTransactionCorrection }
  | { type: 'execute-sales-credit'; correction: PostedTransactionCorrection }
  | null;

const correctionTypeLabels: Record<PostedTransactionCorrectionType, string> = {
  MISSING_PAYMENT_PROOF: 'Bukti pembayaran belum atau perlu dilengkapi',
  PAYMENT_DATE: 'Tanggal pembayaran tidak sesuai',
  PAYMENT_AMOUNT: 'Nominal pembayaran tidak sesuai',
  SOURCE_BANK: 'Rekening sumber tidak sesuai',
  DESTINATION_ACCOUNT: 'Rekening tujuan atau penerima tidak sesuai',
  PURCHASE_DATE: 'Tanggal pembelian tidak sesuai',
  SALES_CREDIT_CLASSIFICATION: 'Reklasifikasi penjualan kredit lama',
  COA_RECLASSIFICATION: 'Reklasifikasi COA transaksi terposting',
  OTHER: 'Koreksi lain',
};

const correctionTypeOptions: Array<{ value: PostedTransactionCorrectionType; label: string }> = [
  { value: 'MISSING_PAYMENT_PROOF', label: correctionTypeLabels.MISSING_PAYMENT_PROOF },
  { value: 'COA_RECLASSIFICATION', label: correctionTypeLabels.COA_RECLASSIFICATION },
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
  correctionTypeLabels[type as PostedTransactionCorrectionType] || type.replace(/_/g, ' ');

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
  const canRequest = Boolean(
    target &&
      target.allowCreate !== false &&
      (isFinanceStaffRole(role) || isAccountingRole(role) || isFinanceManagerRole(role) || role === 'admin'),
  );
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
  const isSalesCreditCorrection = actionModal?.correction.correction_type === 'SALES_CREDIT_CLASSIFICATION';
  const isCOAReclassification = actionModal?.correction.correction_type === 'COA_RECLASSIFICATION';

  const purchaseID = target?.purchaseId;
  const paymentID = target?.paymentId;
  const saleID = target?.saleId;
  const hasTarget = Boolean(target);

  const loadCorrections = useCallback(async () => {
    if (!token || !canAccessWorkflow) return;
    setLoading(true);
    try {
      const response = await postedTransactionCorrectionService.list(token, {
        purchase_id: purchaseID,
        payment_id: paymentID,
        sale_id: saleID,
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
  }, [canAccessWorkflow, hasTarget, paymentID, purchaseID, saleID, toast, token]);

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
    setReviewResolution(
      correction.correction_type === 'SALES_CREDIT_CLASSIFICATION'
        ? 'JOURNAL_RECLASSIFICATION'
        : correction.accounting_resolution || 'JOURNAL_RECLASSIFICATION',
    );
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

  const openSalesCreditExecution = (correction: PostedTransactionCorrection) => {
    setActionModal({ type: 'execute-sales-credit', correction });
  };

  const handleAccountingReview = async () => {
    if (!actionModal || actionModal.type !== 'accounting' || !token) return;
    if (!reviewNotes.trim()) {
      toast({ title: 'Catatan Accounting wajib diisi', status: 'warning', duration: 3500 });
      return;
    }
    if (isSalesCreditCorrection && !proposedAccountId) {
      toast({ title: 'Pilih COA piutang terlebih dahulu', status: 'warning', duration: 3500 });
      return;
    }
    setSaving(true);
    try {
      await postedTransactionCorrectionService.reviewAccounting(token, actionModal.correction.id, {
        resolution: isSalesCreditCorrection ? 'JOURNAL_RECLASSIFICATION' : reviewResolution,
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
    const autoCompletesCOAReclassification = actionModal.correction.correction_type === 'COA_RECLASSIFICATION';
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
        title: actionModal.approve
          ? autoCompletesCOAReclassification
            ? 'Reklasifikasi COA berhasil diposting'
            : 'Koreksi disetujui untuk dieksekusi'
          : 'Koreksi ditolak',
        description: actionModal.approve && autoCompletesCOAReclassification
          ? 'Jurnal reklasifikasi baru telah dibuat. Jurnal asal dan kas/bank tetap tidak berubah.'
          : undefined,
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

  const handleSalesCreditExecution = async () => {
    if (!actionModal || actionModal.type !== 'execute-sales-credit' || !token) return;
    setSaving(true);
    try {
      await postedTransactionCorrectionService.executeSalesCreditCorrection(token, actionModal.correction.id);
      toast({
        title: 'Reklasifikasi piutang berhasil dibuat',
        description: 'Jurnal asal tetap tersimpan dan penerimaan pelanggan telah ditautkan ke invoice.',
        status: 'success',
        duration: 5000,
      });
      setActionModal(null);
      await refreshAfterChange();
    } catch (error: unknown) {
      toast({ title: 'Reklasifikasi piutang gagal', description: getErrorDescription(error), status: 'error', duration: 6000 });
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
              Ajukan Koreksi Transaksi Terposting
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
            const isSalesCreditClassification = correction.correction_type === 'SALES_CREDIT_CLASSIFICATION';
            const isCOAReclassification = correction.correction_type === 'COA_RECLASSIFICATION';
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
                      isSalesCreditClassification ? (
                        <Button size="xs" colorScheme="green" leftIcon={<FiCheckCircle />} onClick={() => openSalesCreditExecution(correction)}>
                          Eksekusi Reklasifikasi Piutang
                        </Button>
                      ) : (
                        <Button size="xs" colorScheme="green" leftIcon={<FiCheckCircle />} onClick={() => openCompletion(correction)}>
                          Tandai Selesai
                        </Button>
                      )
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

                {isSalesCreditClassification && correction.linked_payment && (
                  <Text mt={3} fontSize="sm" color="gray.600">
                    Penerimaan yang akan ditautkan: {correction.linked_payment.code} ({formatCurrency(correction.linked_payment.amount)})
                  </Text>
                )}
                {isCOAReclassification && (
                  <Text mt={3} fontSize="sm" color="gray.600">
                    COA asal: {correction.original_account?.code || '-'} - {correction.original_account?.name || '-'}
                    {' '}ke COA pengganti: {correction.proposed_account?.code || '-'} - {correction.proposed_account?.name || '-'}
                  </Text>
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
        canRequestCOAReclassification={canReviewAccounting && Boolean(target?.purchaseId)}
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
            {actionModal?.type === 'finance' && (actionModal.approve ? isCOAReclassification ? 'Setujui Reklasifikasi COA' : 'Setujui Koreksi Transaksi' : 'Tolak Koreksi Transaksi')}
            {actionModal?.type === 'complete' && 'Catat Penyelesaian Koreksi'}
            {actionModal?.type === 'execute-sales-credit' && 'Eksekusi Reklasifikasi Piutang'}
          </ModalHeader>
          <ModalCloseButton isDisabled={saving} />
          <ModalBody flex="1" minH="0" overflowY="auto">
            {actionModal?.type === 'accounting' && (
              <VStack align="stretch" spacing={4}>
                <Alert status="info" variant="left-accent"><AlertIcon /><AlertDescription>{isSalesCreditCorrection ? 'Accounting memilih COA piutang yang sudah dikredit oleh penerimaan pelanggan. Eksekusi membuat jurnal Dr Piutang / Cr Kas-Bank asal dan satu mutasi koreksi audit, tanpa membuat penerimaan baru.' : 'Accounting menentukan perlakuan koreksi. Tidak ada jurnal yang dibuat otomatis pada tahap ini.'}</AlertDescription></Alert>
                {isSalesCreditCorrection ? (
                  <Box borderWidth="1px" borderRadius="md" p={3}>
                    <Text fontWeight="semibold">Rencana penyelesaian</Text>
                    <Text fontSize="sm" color="gray.600">Jurnal reklasifikasi piutang</Text>
                  </Box>
                ) : (
                  <FormControl isRequired>
                    <FormLabel>Rencana penyelesaian</FormLabel>
                    <Select value={reviewResolution} onChange={(event) => setReviewResolution(event.target.value as PostedTransactionCorrectionResolution)}>
                      {resolutionOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                    </Select>
                  </FormControl>
                )}
                <FormControl isRequired={isSalesCreditCorrection}>
                  <FormLabel>{isSalesCreditCorrection ? 'COA Piutang Usaha' : 'COA usulan (opsional)'}</FormLabel>
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
                  <FormHelperText>{isSalesCreditCorrection ? 'Pilih akun piutang yang sama dengan akun kredit pada penerimaan pelanggan yang dipilih.' : 'Isi bila Accounting sudah mengetahui akun yang akan digunakan saat eksekusi koreksi.'}</FormHelperText>
                </FormControl>
                <FormControl isRequired>
                  <FormLabel>Catatan Accounting</FormLabel>
                  <Textarea value={reviewNotes} onChange={(event) => setReviewNotes(event.target.value)} rows={5} placeholder="Jelaskan perlakuan koreksi, dasar bukti, dan langkah eksekusinya." />
                </FormControl>
              </VStack>
            )}
            {actionModal?.type === 'finance' && (
              <VStack align="stretch" spacing={4}>
                <Alert status={actionModal.approve ? 'success' : 'warning'} variant="left-accent"><AlertIcon /><AlertDescription>{actionModal.approve ? isCOAReclassification ? 'Persetujuan akan langsung membuat jurnal reklasifikasi COA baru. Jurnal asal dan mutasi kas/bank tetap tidak diubah.' : 'Persetujuan hanya membuka koreksi untuk dieksekusi oleh Accounting. Transaksi dan jurnal asal tetap tidak diubah.' : 'Penolakan akan menutup pengajuan ini tanpa mengubah transaksi asal.'}</AlertDescription></Alert>
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
            {actionModal?.type === 'execute-sales-credit' && (
              <VStack align="stretch" spacing={4}>
                <Alert status="warning" variant="left-accent"><AlertIcon /><AlertDescription>Aksi ini membuat satu jurnal koreksi terposting: debit Piutang Usaha dan kredit Kas/Bank yang salah tercatat pada invoice. Sistem juga membuat satu mutasi koreksi audit untuk membalik penerimaan awal yang salah, tanpa mencatat penerimaan atau pengeluaran uang baru.</AlertDescription></Alert>
                <Box borderWidth="1px" borderRadius="md" p={3} fontSize="sm">
                  <Text fontWeight="semibold">{actionModal.correction.target_code}</Text>
                  <Text color="gray.600">COA piutang: {actionModal.correction.proposed_account?.code || '-'} - {actionModal.correction.proposed_account?.name || '-'}</Text>
                  <Text color="gray.600">Penerimaan: {actionModal.correction.linked_payment?.code || '-'}</Text>
                </Box>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={() => setActionModal(null)} isDisabled={saving}>Batal</Button>
            {actionModal?.type === 'accounting' && <Button colorScheme="blue" onClick={() => void handleAccountingReview()} isLoading={saving}>Kirim ke Finance Manager</Button>}
            {actionModal?.type === 'finance' && <Button colorScheme={actionModal.approve ? 'green' : 'red'} onClick={() => void handleFinanceDecision()} isLoading={saving}>{actionModal.approve ? 'Setujui Koreksi' : 'Tolak Koreksi'}</Button>}
            {actionModal?.type === 'complete' && <Button colorScheme="green" onClick={() => void handleComplete()} isLoading={saving}>Tandai Selesai</Button>}
            {actionModal?.type === 'execute-sales-credit' && <Button colorScheme="green" onClick={() => void handleSalesCreditExecution()} isLoading={saving}>Eksekusi Jurnal Koreksi</Button>}
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
  canRequestCOAReclassification: boolean;
  onSuccess: () => Promise<void>;
}

const CreateCorrectionModal: React.FC<CreateCorrectionModalProps> = ({
  isOpen,
  onClose,
  target,
  token,
  canRequestCOAReclassification,
  onSuccess,
}) => {
  const toast = useToast();
  const sourceJournalBackground = useColorModeValue('gray.50', 'whiteAlpha.100');
  const [saving, setSaving] = useState(false);
  const [correctionType, setCorrectionType] = useState<PostedTransactionCorrectionType>('MISSING_PAYMENT_PROOF');
  const [reason, setReason] = useState('');
  const [requestedDate, setRequestedDate] = useState('');
  const [reference, setReference] = useState('');
  const [document, setDocument] = useState<File | null>(null);
  const [paymentCandidates, setPaymentCandidates] = useState<LegacySalesCreditPaymentCandidate[]>([]);
  const [loadingPaymentCandidates, setLoadingPaymentCandidates] = useState(false);
  const [linkedPaymentId, setLinkedPaymentId] = useState<number | undefined>();
  const [coaJournalLineCandidates, setCOAJournalLineCandidates] = useState<PurchaseCOAReclassificationJournalLineCandidate[]>([]);
  const [coaAccounts, setCOAAccounts] = useState<COAReclassificationAccountCandidate[]>([]);
  const [loadingCOAOptions, setLoadingCOAOptions] = useState(false);
  const [selectedOriginalJournalLineIds, setSelectedOriginalJournalLineIds] = useState<number[]>([]);
  const [proposedAccountId, setProposedAccountId] = useState<number | undefined>();
  const isSalesCreditCorrection = Boolean(target?.saleId);
  const isPurchaseCOAReclassification = correctionType === 'COA_RECLASSIFICATION';
  const availableCorrectionTypeOptions = correctionTypeOptions.filter(
    (option) => option.value !== 'COA_RECLASSIFICATION' || canRequestCOAReclassification,
  );

  useEffect(() => {
    if (!isOpen) return;
    setCorrectionType(isSalesCreditCorrection ? 'SALES_CREDIT_CLASSIFICATION' : 'MISSING_PAYMENT_PROOF');
    setReason('');
    setRequestedDate('');
    setReference('');
    setDocument(null);
    setLinkedPaymentId(undefined);
    setPaymentCandidates([]);
    setCOAJournalLineCandidates([]);
    setCOAAccounts([]);
    setSelectedOriginalJournalLineIds([]);
    setProposedAccountId(undefined);
  }, [isOpen, isSalesCreditCorrection]);

  useEffect(() => {
    if (!isOpen || !isSalesCreditCorrection || !target?.saleId || !token) return;
    let cancelled = false;
    setLoadingPaymentCandidates(true);
    postedTransactionCorrectionService
      .getLegacySalesCreditPaymentCandidates(token, target.saleId)
      .then((candidates) => {
        if (!cancelled) setPaymentCandidates(candidates);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          toast({
            title: 'Penerimaan pelanggan tidak dapat dimuat',
            description: getErrorDescription(error),
            status: 'error',
            duration: 5000,
          });
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingPaymentCandidates(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, isSalesCreditCorrection, target?.saleId, toast, token]);

  useEffect(() => {
    if (!isOpen || !isPurchaseCOAReclassification || !target?.purchaseId || !token) return;
    let cancelled = false;
    setLoadingCOAOptions(true);
    Promise.all([
      postedTransactionCorrectionService.getPurchaseCOAReclassificationCandidates(token, target.purchaseId),
      postedTransactionCorrectionService.getCOAReclassificationAccounts(token),
    ])
      .then(([lines, accounts]) => {
        if (cancelled) return;
        setCOAJournalLineCandidates(lines);
        setCOAAccounts(accounts);
      })
      .catch((error: unknown) => {
        if (!cancelled) {
          toast({
            title: 'Data koreksi COA tidak dapat dimuat',
            description: getErrorDescription(error),
            status: 'error',
            duration: 5000,
          });
        }
      })
      .finally(() => {
        if (!cancelled) setLoadingCOAOptions(false);
      });
    return () => {
      cancelled = true;
    };
  }, [isOpen, isPurchaseCOAReclassification, target?.purchaseId, toast, token]);

  const wantsDate = isSalesCreditCorrection;
  const sourceJournal = coaJournalLineCandidates[0];
  const selectedOriginalAccountIds = new Set(
    coaJournalLineCandidates
      .filter((line) => selectedOriginalJournalLineIds.includes(line.id))
      .map((line) => line.account_id),
  );
  const bulkSelectableLineIds = coaJournalLineCandidates
    .filter((line) => line.bulk_selectable)
    .map((line) => line.id);
  const allBulkSelectableLinesSelected = bulkSelectableLineIds.length > 0
    && bulkSelectableLineIds.every((lineId) => selectedOriginalJournalLineIds.includes(lineId));

  const toggleCOAJournalLine = (lineId: number) => {
    setSelectedOriginalJournalLineIds((current) => (
      current.includes(lineId)
        ? current.filter((id) => id !== lineId)
        : [...current, lineId]
    ));
    setProposedAccountId(undefined);
  };

  const toggleAllBulkSelectableLines = () => {
    setSelectedOriginalJournalLineIds((current) => {
      if (allBulkSelectableLinesSelected) {
        return current.filter((lineId) => !bulkSelectableLineIds.includes(lineId));
      }
      return Array.from(new Set([...current, ...bulkSelectableLineIds]));
    });
    setProposedAccountId(undefined);
  };

  const handleSubmit = async () => {
    if (!target || !token) return;
    if (!reason.trim()) {
      toast({ title: 'Alasan koreksi wajib diisi', status: 'warning', duration: 3500 });
      return;
    }
    if (isSalesCreditCorrection && (!requestedDate || !linkedPaymentId)) {
      toast({ title: 'Tanggal koreksi dan penerimaan pelanggan wajib dipilih', status: 'warning', duration: 3500 });
      return;
    }
    if (isPurchaseCOAReclassification && (selectedOriginalJournalLineIds.length === 0 || !proposedAccountId)) {
      toast({ title: 'Minimal satu baris jurnal asal dan COA pengganti wajib dipilih', status: 'warning', duration: 3500 });
      return;
    }
    setSaving(true);
    try {
      const payload: CreatePostedTransactionCorrectionRequest = {
        purchase_id: target.purchaseId,
        payment_id: target.paymentId,
        sale_id: target.saleId,
        linked_payment_id: linkedPaymentId,
        original_journal_line_ids: isPurchaseCOAReclassification ? selectedOriginalJournalLineIds : undefined,
        proposed_account_id: isPurchaseCOAReclassification ? proposedAccountId : undefined,
        correction_type: correctionType,
        reason: reason.trim(),
        requested_date: requestedDate || undefined,
        requested_reference: reference.trim() || undefined,
      };
      const correction = await postedTransactionCorrectionService.create(token, payload);
      if (document) {
        await postedTransactionCorrectionService.uploadDocument(token, correction.id, document);
      }
      toast({
        title: 'Pengajuan koreksi dikirim',
        description: isPurchaseCOAReclassification
          ? 'Koreksi COA sekarang menunggu persetujuan Finance Manager.'
          : 'Pengajuan sekarang menunggu review Accounting.',
        status: 'success',
        duration: 4000,
      });
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
            <Alert status="warning" variant="left-accent"><AlertIcon /><AlertDescription>{isSalesCreditCorrection ? 'Pengajuan ini hanya untuk sale kredit lama yang salah tercatat sebagai kas/bank. Sistem akan menolak jika penerimaan pelanggan belum memenuhi syarat audit.' : isPurchaseCOAReclassification ? 'Accounting memilih satu atau beberapa baris jurnal pembelian dan satu COA pengganti. Pengajuan langsung menunggu Finance Manager. Jika disetujui, sistem membuat jurnal reklasifikasi baru tanpa mengubah jurnal asal atau mutasi kas/bank.' : 'Pengajuan ini hanya untuk melengkapi bukti pembayaran dan tidak mengubah transaksi atau jurnal asal.'}</AlertDescription></Alert>
            <Box borderWidth="1px" borderRadius="md" p={3}>
              <Text fontWeight="semibold">Transaksi asal</Text>
              <Text fontSize="sm" color="gray.500">{target?.code || '-'}</Text>
            </Box>
            {isSalesCreditCorrection ? (
              <Box borderWidth="1px" borderRadius="md" p={3}>
                <Text fontWeight="semibold">Jenis koreksi</Text>
                <Text fontSize="sm" color="gray.600">Reklasifikasi penjualan kredit lama</Text>
              </Box>
            ) : (
              <FormControl isRequired>
                <FormLabel>Jenis koreksi</FormLabel>
                <Select value={correctionType} onChange={(event) => setCorrectionType(event.target.value as PostedTransactionCorrectionType)}>
                  {availableCorrectionTypeOptions.map((option) => <option key={option.value} value={option.value}>{option.label}</option>)}
                </Select>
              </FormControl>
            )}
            {wantsDate && <FormControl isRequired><FormLabel>{isSalesCreditCorrection ? 'Tanggal jurnal koreksi' : 'Tanggal aktual'}</FormLabel><Input type="date" value={requestedDate} onChange={(event) => setRequestedDate(event.target.value)} /></FormControl>}
            {isSalesCreditCorrection && (
              <FormControl isRequired>
                <FormLabel>Penerimaan pelanggan yang sudah ada</FormLabel>
                <SearchableSelect
                  options={paymentCandidates.map((payment) => ({
                    id: payment.id,
                    code: payment.code,
                    name: `${formatCurrency(payment.amount)} | ${new Date(payment.date).toLocaleDateString('id-ID')} | ${payment.cash_bank_name || payment.cash_bank_code || 'Kas/Bank'}`,
                  }))}
                  value={linkedPaymentId}
                  onChange={(value) => {
                    const paymentID = Number(value);
                    setLinkedPaymentId(Number.isInteger(paymentID) && paymentID > 0 ? paymentID : undefined);
                  }}
                  placeholder={loadingPaymentCandidates ? 'Memuat penerimaan pelanggan...' : 'Cari kode atau nilai penerimaan'}
                  isLoading={loadingPaymentCandidates}
                  allowClear
                />
                <FormHelperText>Hanya penerimaan pelanggan yang belum dialokasikan dan memiliki jurnal terposting yang dapat dipilih.</FormHelperText>
                {!loadingPaymentCandidates && paymentCandidates.length === 0 && <Text mt={2} fontSize="sm" color="orange.600">Tidak ada penerimaan yang memenuhi syarat koreksi otomatis. Gunakan jurnal koreksi manual setelah review Accounting.</Text>}
              </FormControl>
            )}
            {isPurchaseCOAReclassification && (
              <VStack align="stretch" spacing={4} borderWidth="1px" borderRadius="md" p={3}>
                <Text fontWeight="semibold">Reklasifikasi COA</Text>
                <Box borderWidth="1px" borderRadius="md" p={3} bg={sourceJournalBackground}>
                  <Text fontWeight="semibold" fontSize="sm">Jurnal asal (otomatis)</Text>
                  <Text fontSize="sm" color="gray.600">
                    {sourceJournal
                      ? `${sourceJournal.journal_entry_number || `#${sourceJournal.journal_entry_id}`} | ${new Date(sourceJournal.journal_entry_date).toLocaleDateString('id-ID')}`
                      : loadingCOAOptions ? 'Memuat jurnal pembelian...' : 'Jurnal pembelian belum tersedia'}
                  </Text>
                  <Text mt={1} fontSize="xs" color="gray.500">Jurnal sumber ditentukan dari transaksi pembelian dan tidak perlu diinput ulang.</Text>
                </Box>
                <FormControl isRequired>
                  <HStack justify="space-between" align="center" mb={2}>
                    <FormLabel mb={0}>Baris jurnal asal</FormLabel>
                    <Button size="sm" variant="outline" onClick={toggleAllBulkSelectableLines} isDisabled={loadingCOAOptions || bulkSelectableLineIds.length === 0}>
                      {allBulkSelectableLinesSelected ? 'Batalkan pilih semua' : 'Pilih semua baris item'}
                    </Button>
                  </HStack>
                  <Box borderWidth="1px" borderRadius="md" maxH="240px" overflowY="auto" p={2}>
                    <VStack align="stretch" spacing={1}>
                      {coaJournalLineCandidates.map((line) => (
                        <Checkbox
                          key={line.id}
                          isChecked={selectedOriginalJournalLineIds.includes(line.id)}
                          onChange={() => toggleCOAJournalLine(line.id)}
                          alignItems="flex-start"
                          py={1}
                        >
                          <Box>
                            <Text fontSize="sm" fontWeight="medium">
                              Baris {line.line_number}: {line.account_code} - {line.account_name}
                            </Text>
                            <Text fontSize="xs" color="gray.500">
                              {line.debit_amount > 0 ? `Debit ${formatCurrency(line.debit_amount)}` : `Kredit ${formatCurrency(line.credit_amount)}`}
                              {line.description ? ` | ${line.description}` : ''}
                            </Text>
                          </Box>
                        </Checkbox>
                      ))}
                      {!loadingCOAOptions && coaJournalLineCandidates.length === 0 && <Text fontSize="sm" color="orange.600">Tidak ada baris jurnal pembelian yang memenuhi syarat untuk koreksi COA otomatis.</Text>}
                    </VStack>
                  </Box>
                  <FormHelperText>{selectedOriginalJournalLineIds.length} baris dipilih. Pilih semua hanya mencentang baris debit item; PPN dan baris lain tetap dapat dipilih satu per satu bila diperlukan.</FormHelperText>
                </FormControl>
                <FormControl isRequired>
                  <FormLabel>COA pengganti</FormLabel>
                  <SearchableSelect
                    options={coaAccounts
                      .filter((account) => !selectedOriginalAccountIds.has(account.id))
                      .map((account) => ({ id: account.id, code: account.code, name: account.name }))}
                    value={proposedAccountId}
                    onChange={(value) => {
                      const accountID = Number(value);
                      setProposedAccountId(Number.isInteger(accountID) && accountID > 0 ? accountID : undefined);
                    }}
                    placeholder={loadingCOAOptions ? 'Memuat daftar COA...' : 'Cari kode atau nama COA pengganti'}
                    isLoading={loadingCOAOptions}
                    allowClear
                  />
                  <FormHelperText>Satu COA pengganti dipakai untuk seluruh baris dalam pengajuan ini. Buat pengajuan terpisah bila target COA-nya berbeda.</FormHelperText>
                </FormControl>
              </VStack>
            )}
            <FormControl><FormLabel>Referensi aktual (opsional)</FormLabel><Input value={reference} onChange={(event) => setReference(event.target.value)} placeholder="Nomor Kopra, invoice, atau referensi lain" /></FormControl>
            <FormControl isRequired>
              <FormLabel>Alasan dan kronologi</FormLabel>
              <Textarea value={reason} onChange={(event) => setReason(event.target.value)} rows={5} placeholder={isSalesCreditCorrection ? "Jelaskan bahwa invoice kredit dahulu tercatat sebagai kas/bank dan sebutkan dasar penerimaan pelanggan yang dipilih." : isPurchaseCOAReclassification ? "Jelaskan mengapa akun asal tidak tepat dan dasar pemilihan COA pengganti." : "Jelaskan data yang salah, data aktual, dan dasar buktinya."} />
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
          <Button colorScheme="orange" leftIcon={<FiEdit3 />} onClick={() => void handleSubmit()} isLoading={saving}>{isSalesCreditCorrection ? 'Ajukan Reklasifikasi' : isPurchaseCOAReclassification ? 'Kirim ke Finance Manager' : 'Kirim ke Accounting'}</Button>
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
