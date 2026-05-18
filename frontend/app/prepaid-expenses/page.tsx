'use client';

import { useState, useEffect, useRef } from 'react';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import { isFinanceManagerRole, normalizeRole } from '@/utils/roles';
import {
  AlertDialog,
  AlertDialogBody,
  AlertDialogContent,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogOverlay,
  Box,
  Button,
  Flex,
  Heading,
  Icon,
  Table,
  Tbody,
  Td,
  Th,
  Thead,
  Tr,
  Badge,
  useDisclosure,
  useToast,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Text,
  FormControl,
  FormLabel,
  Input,
  VStack,
  HStack,
  Divider,
  Select,
  useColorModeValue
} from '@chakra-ui/react';
import { FiRefreshCw, FiEye, FiCheckCircle, FiXCircle, FiTrash2, FiEdit2 } from 'react-icons/fi';
import api from '@/services/api';

interface PrepaidExpense {
  id: number;
  purchase_id: number;
  purchase_code?: string;
  name: string;
  description: string;
  total_amount: number;
  amortization_month: number;
  monthly_amount: number;
  start_date: string;
  end_date: string;
  status: string;
  prepaid_account_id: number;
  expense_account_id: number;
}

interface PrepaidSchedule {
  id: number;
  prepaid_id: number;
  month_number: number;
  scheduled_date: string;
  amount: number;
  status: string;
  journal_entry_id: number | null;
}

interface Account {
  id: number;
  code: string;
  name: string;
  type: string;
  category: string;
  is_header?: boolean;
}

export default function PrepaidExpensesPage() {
  const { user } = useAuth();
  const normalizedRole = normalizeRole(user?.role);
  const isAdmin = normalizedRole === 'admin';
  const isFinanceManager = isFinanceManagerRole(normalizedRole);
  const {
    canEdit: canEditPrepaidExpenses,
    canDelete: canDeletePrepaidExpenses,
  } = useModulePermissions('prepaid_expenses');
  const { canView: canViewAccounts } = useModulePermissions('accounts');
  const canManagePrepaidExpenses = isAdmin || isFinanceManager || canEditPrepaidExpenses;
  const canSelectPrepaidAccounts = isAdmin || isFinanceManager || canViewAccounts;
  const [prepaids, setPrepaids] = useState<PrepaidExpense[]>([]);
  const [schedules, setSchedules] = useState<PrepaidSchedule[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedPrepaid, setSelectedPrepaid] = useState<PrepaidExpense | null>(null);
  const [loading, setLoading] = useState(false);

  const { isOpen, onOpen, onClose } = useDisclosure();
  const { isOpen: isTerminateOpen, onOpen: onTerminateOpen, onClose: onTerminateClose } = useDisclosure();
  const { isOpen: isDeleteOpen, onOpen: onDeleteOpen, onClose: onDeleteClose } = useDisclosure();
  const { isOpen: isEditScheduleOpen, onOpen: onEditScheduleOpen, onClose: onEditScheduleClose } = useDisclosure();
  const [terminateType, setTerminateType] = useState('REFUND');
  const [destinationAccount, setDestinationAccount] = useState('');
  const [prepaidToTerminate, setPrepaidToTerminate] = useState<PrepaidExpense | null>(null);
  const [prepaidToDelete, setPrepaidToDelete] = useState<PrepaidExpense | null>(null);
  const [isDeleteSubmitting, setIsDeleteSubmitting] = useState(false);
  const [scheduleToEdit, setScheduleToEdit] = useState<PrepaidSchedule | null>(null);
  const [editScheduledDate, setEditScheduledDate] = useState('');
  const [isScheduleSubmitting, setIsScheduleSubmitting] = useState(false);
  const cancelDeleteRef = useRef<HTMLButtonElement | null>(null);
  const toast = useToast();
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const textColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const cardBg = useColorModeValue('white', 'gray.800');
  const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const inputBg = useColorModeValue('white', 'whiteAlpha.50');
  const inputBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalFooterBg = useColorModeValue('gray.50', 'whiteAlpha.50');
  const fieldFocusStyles = {
    borderColor: 'blue.400',
    boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)',
  };

  const fetchPrepaids = async () => {
    setLoading(true);
    try {
      const response = await api.get('/api/v1/prepaid');
      setPrepaids(response.data.data || []);
    } catch (error: any) {
      toast({
        title: 'Error fetch',
        description: error.response?.data?.error || 'Gagal mengambil data prepaid',
        status: 'error',
        duration: 3000
      });
    } finally {
      setLoading(false);
    }
  };

  const fetchAccounts = async () => {
    try {
      const response = await api.get('/api/v1/accounts');
      setAccounts(response.data.data || []);
    } catch (error) {
      console.error('Failed to fetch accounts', error);
    }
  };

  const loadPrepaidDetails = async (prepaid: PrepaidExpense) => {
    setSelectedPrepaid(prepaid);
    try {
      const response = await api.get(`/api/v1/prepaid/${prepaid.id}`);
      const prepaidData = response.data.data;
      setSelectedPrepaid(prepaidData);
      setSchedules(prepaidData.schedules || []);
      onOpen();
    } catch {
      toast({
        title: 'Error',
        description: 'Gagal memuat jadwal amortisasi',
        status: 'error',
        duration: 3000
      });
    }
  };

  const toDateInputValue = (dateString: string) => {
    const date = new Date(dateString);
    if (Number.isNaN(date.getTime())) {
      return '';
    }

    const timezoneOffset = date.getTimezoneOffset() * 60000;
    return new Date(date.getTime() - timezoneOffset).toISOString().slice(0, 10);
  };

  const handleAmortize = async (scheduleId: number) => {
    if (!selectedPrepaid) return;
    if (!canManagePrepaidExpenses) {
      toast({
        title: 'Akses ditolak',
        description: 'Anda belum memiliki permission edit untuk prepaid expense.',
        status: 'warning',
        duration: 3000
      });
      return;
    }

    // Periksa apakah expense account sudah diset
    if (!selectedPrepaid.expense_account_id || selectedPrepaid.expense_account_id === 0) {
      toast({
        title: 'Akun Beban Belum Diset',
        description: 'Pilih Akun Beban (Expense Account) terlebih dahulu di bagian atas sebelum mengeksekusi jurnal',
        status: 'warning',
        duration: 5000
      });
      return;
    }

    try {
      await api.post(`/api/v1/prepaid/${selectedPrepaid.id}/amortize/${scheduleId}`);
      toast({
        title: 'Sukses',
        description: 'Jurnal Amortisasi berhasil dibuat',
        status: 'success',
        duration: 3000
      });
      // Refresh detail
      loadPrepaidDetails(selectedPrepaid);
      fetchPrepaids();
    } catch (error: any) {
      toast({
        title: 'Gagal Eksekusi',
        description: error.response?.data?.error || 'Terjadi kesalahan sistem',
        status: 'error',
        duration: 5000
      });
    }
  };

  const handleSetExpenseAccount = async (e: React.ChangeEvent<HTMLSelectElement>) => {
    if (!selectedPrepaid) return;
    if (!canManagePrepaidExpenses || !canSelectPrepaidAccounts) {
      toast({
        title: 'Akses ditolak',
        description: 'Anda membutuhkan akses edit prepaid expense dan view COA untuk memilih akun beban.',
        status: 'warning',
        duration: 3000
      });
      return;
    }
    const accountId = parseInt(e.target.value);
    if (!accountId) return;

    try {
      await api.put(`/api/v1/prepaid/${selectedPrepaid.id}/account`, {
        expense_account_id: accountId
      });

      setSelectedPrepaid({ ...selectedPrepaid, expense_account_id: accountId });

      toast({
        title: 'Sukses',
        description: 'Akun beban amortisasi berhasil diupdate',
        status: 'success',
        duration: 2000
      });
      fetchPrepaids();
    } catch (error: any) {
      toast({
        title: 'Gagal update akun',
        description: error.response?.data?.error || 'Kesalahan sistem',
        status: 'error',
        duration: 3000
      });
    }
  };

  const handleTerminateClick = (prepaid: PrepaidExpense) => {
    if (!canManagePrepaidExpenses) {
      toast({
        title: 'Akses ditolak',
        description: 'Anda belum memiliki permission edit untuk terminasi prepaid expense.',
        status: 'warning',
        duration: 3000
      });
      return;
    }
    setPrepaidToTerminate(prepaid);
    setTerminateType('REFUND');
    setDestinationAccount('');
    onTerminateOpen();
  };

  const handleTerminateSubmit = async () => {
    if (!prepaidToTerminate) return;
    if (!canManagePrepaidExpenses) {
      toast({
        title: 'Akses ditolak',
        description: 'Anda belum memiliki permission edit untuk terminasi prepaid expense.',
        status: 'warning',
        duration: 3000
      });
      return;
    }

    // Validasi akun tujuan
    if (!destinationAccount || parseInt(destinationAccount) === 0) {
      toast({
        title: 'Akun Tujuan Kosong',
        description: 'Pilih Akun Tujuan untuk mencatat jurnal terminasi',
        status: 'warning',
        duration: 3000
      });
      return;
    }

    try {
      await api.post(`/api/v1/prepaid/${prepaidToTerminate.id}/terminate`, {
        terminate_type: terminateType,
        destination_account_id: parseInt(destinationAccount)
      });

      toast({
        title: 'Sukses Terminasi',
        description: 'Prepaid berhasil diterminasi',
        status: 'success',
        duration: 3000
      });
      onTerminateClose();
      fetchPrepaids();
    } catch (error: any) {
      toast({
        title: 'Gagal Terminasi',
        description: error.response?.data?.error || 'Terjadi kesalahan sistem',
        status: 'error',
        duration: 5000
      });
    }
  };

  const handleEditScheduleClick = (schedule: PrepaidSchedule) => {
    if (!canManagePrepaidExpenses) {
      toast({
        title: 'Akses ditolak',
        description: 'Anda belum memiliki permission edit untuk mengubah jadwal prepaid expense.',
        status: 'warning',
        duration: 3000
      });
      return;
    }
    setScheduleToEdit(schedule);
    setEditScheduledDate(toDateInputValue(schedule.scheduled_date));
    onEditScheduleOpen();
  };

  const handleUpdateScheduleSubmit = async () => {
    if (!selectedPrepaid || !scheduleToEdit) return;
    if (!canManagePrepaidExpenses) {
      toast({
        title: 'Akses ditolak',
        description: 'Anda belum memiliki permission edit untuk mengubah jadwal prepaid expense.',
        status: 'warning',
        duration: 3000
      });
      return;
    }
    if (!editScheduledDate) {
      toast({
        title: 'Tanggal belum diisi',
        description: 'Pilih tanggal jadwal amortisasi terlebih dahulu',
        status: 'warning',
        duration: 3000
      });
      return;
    }

    setIsScheduleSubmitting(true);
    try {
      await api.put(`/api/v1/prepaid/${selectedPrepaid.id}/schedules/${scheduleToEdit.id}`, {
        scheduled_date: editScheduledDate
      });

      toast({
        title: 'Sukses',
        description: 'Jadwal prepaid expense berhasil diperbarui',
        status: 'success',
        duration: 3000
      });

      await loadPrepaidDetails(selectedPrepaid);
      fetchPrepaids();
      onEditScheduleClose();
      setScheduleToEdit(null);
    } catch (error: any) {
      toast({
        title: 'Gagal update jadwal',
        description: error.response?.data?.details || error.response?.data?.error || 'Terjadi kesalahan sistem',
        status: 'error',
        duration: 5000
      });
    } finally {
      setIsScheduleSubmitting(false);
    }
  };

  const handleDeleteClick = (prepaid: PrepaidExpense) => {
    setPrepaidToDelete(prepaid);
    onDeleteOpen();
  };

  const handleDeleteSubmit = async () => {
    if (!prepaidToDelete) return;

    setIsDeleteSubmitting(true);
    try {
      await api.delete(`/api/v1/prepaid/${prepaidToDelete.id}`);

      toast({
        title: 'Sukses',
        description: 'Transaksi prepaid berhasil dihapus',
        status: 'success',
        duration: 3000
      });

      if (selectedPrepaid?.id === prepaidToDelete.id) {
        setSelectedPrepaid(null);
        setSchedules([]);
        onClose();
      }

      onDeleteClose();
      setPrepaidToDelete(null);
      fetchPrepaids();
    } catch (error: any) {
      toast({
        title: 'Gagal hapus',
        description: error.response?.data?.error || 'Terjadi kesalahan sistem',
        status: 'error',
        duration: 5000
      });
    } finally {
      setIsDeleteSubmitting(false);
    }
  };

  useEffect(() => {
    fetchPrepaids();
    fetchAccounts();
  }, []);

  const formatCurrency = (val: number) => new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(val);
  const formatDate = (dateString: string) => new Date(dateString).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });

  return (
    <SimpleLayout>
      <Box p={6}>
      <Flex justifyContent="space-between" alignItems="center" mb={6}>
        <Heading size="lg" color={headingColor}>Prepaid Expenses (Amortisasi)</Heading>
        <Button leftIcon={<Icon as={FiRefreshCw} />} onClick={fetchPrepaids} isLoading={loading}>
          Refresh
        </Button>
      </Flex>

      <Box bg={cardBg} p={4} borderRadius="md" shadow="sm" overflowX="auto" borderWidth="1px" borderColor={borderColor}>
        <Table variant="simple" size="sm">
          <Thead bg={tableHeaderBg}>
            <Tr>
              <Th>Item Name</Th>
              <Th>Purchase Info</Th>
              <Th isNumeric>Biaya Total</Th>
              <Th isNumeric>Per Bulan</Th>
              <Th>Durasi</Th>
              <Th>Status</Th>
              <Th>Aksi</Th>
            </Tr>
          </Thead>
          <Tbody>
            {prepaids.map((item) => (
              <Tr key={item.id} _hover={{ bg: rowHoverBg }}>
                <Td fontWeight="bold">{item.name}</Td>
                <Td>PO #{item.purchase_id}</Td>
                <Td isNumeric>{formatCurrency(item.total_amount)}</Td>
                <Td isNumeric color="purple.600" fontWeight="bold">
                  {formatCurrency(item.monthly_amount)}
                </Td>
                <Td>
                  {item.amortization_month} Bulan <br/>
                  <Text fontSize="xs" color={mutedTextColor}>
                    ({formatDate(item.start_date)} - {formatDate(item.end_date)})
                  </Text>
                </Td>
                <Td>
                  <Badge colorScheme={item.status === 'ACTIVE' ? 'purple' : item.status === 'COMPLETED' ? 'green' : 'red'}>
                    {item.status}
                  </Badge>
                </Td>
                <Td>
                  <HStack spacing={2}>
                    <Button size="sm" colorScheme="blue" leftIcon={<Icon as={FiEye} />} onClick={() => loadPrepaidDetails(item)}>
                      Jadwal
                    </Button>
                    {item.status === 'ACTIVE' && canManagePrepaidExpenses && (
                      <Button size="sm" colorScheme="orange" leftIcon={<Icon as={FiXCircle} />} onClick={() => handleTerminateClick(item)}>
                        Terminasi
                      </Button>
                    )}
                    {(isAdmin || canDeletePrepaidExpenses) && (
                      <Button size="sm" colorScheme="red" variant="outline" leftIcon={<Icon as={FiTrash2} />} onClick={() => handleDeleteClick(item)}>
                        Hapus
                      </Button>
                    )}
                  </HStack>
                </Td>
              </Tr>
            ))}
            {prepaids.length === 0 && !loading && (
              <Tr><Td colSpan={7} textAlign="center" py={6} color={mutedTextColor}>Tidak ada data prepaid expense</Td></Tr>
            )}
          </Tbody>
        </Table>
      </Box>

      {/* Modal Detail & Jadwal */}
      <Modal isOpen={isOpen} onClose={onClose} size="4xl">
        <ModalOverlay />
        <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
          <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Jadwal Amortisasi - {selectedPrepaid?.name}</ModalHeader>
          <ModalCloseButton color={textColor} />

          <ModalBody>
            <Flex gap={6} mb={6} bg={panelBg} p={4} borderRadius="md" borderWidth="1px" borderColor={borderColor}>
              <VStack align="start" flex={1} spacing={2}>
                <Text fontWeight="bold" color={mutedTextColor} fontSize="sm">Total Saldo Prepaid</Text>
                <Text fontSize="xl" fontWeight="bold" color={textColor}>{selectedPrepaid && formatCurrency(selectedPrepaid.total_amount)}</Text>
              </VStack>
              <VStack align="start" flex={1} spacing={2}>
                <Text fontWeight="bold" color={mutedTextColor} fontSize="sm">Akun Beban Tujuan (Expense)</Text>
                <Select size="sm" bg={inputBg} borderColor={inputBorderColor} color={textColor}
                  value={selectedPrepaid?.expense_account_id || ''}
                  onChange={handleSetExpenseAccount}
                  isDisabled={!canManagePrepaidExpenses || !canSelectPrepaidAccounts}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                >
                  <option value="">-- Pilih Akun Beban --</option>
                  {accounts
                    .filter((a: Account) => !a.is_header && (a.type === 'EXPENSE' || a.type === 'OTHER_EXPENSE'))
                    .map(acc => (
                      <option key={acc.id} value={acc.id}>{acc.code} - {acc.name}</option>
                    ))}
                </Select>
                {!canSelectPrepaidAccounts && (
                  <Text fontSize="xs" color="orange.500">
                    Akses COA belum aktif untuk memilih akun beban.
                  </Text>
                )}
              </VStack>
            </Flex>

            <Divider mb={4} />

            <Table size="sm" variant="striped">
              <Thead>
                <Tr>
                  <Th>Bulan Ke-</Th>
                  <Th>Jatuh Tempo</Th>
                  <Th isNumeric>Nilai Amortisasi</Th>
                  <Th>Status Eksekusi</Th>
                  <Th>Aksi</Th>
                </Tr>
              </Thead>
              <Tbody>
                {schedules.map((sched) => (
                  <Tr key={sched.id}>
                    <Td fontWeight="bold">{sched.month_number}</Td>
                    <Td>{formatDate(sched.scheduled_date)}</Td>
                    <Td isNumeric>{formatCurrency(sched.amount)}</Td>
                    <Td>
                      {sched.status === 'POSTED' ? (
                         <Badge colorScheme="green" display="flex" alignItems="center" w="fit-content" gap={1}>
                           <Icon as={FiCheckCircle} /> Jurnal #{sched.journal_entry_id}
                         </Badge>
                      ) : sched.status === 'CANCELLED' ? (
                        <Badge colorScheme="red">Dibatalkan</Badge>
                      ) : (
                        <Badge colorScheme="yellow">Pending</Badge>
                      )}
                    </Td>
                    <Td>
                      {sched.status === 'PENDING' && canManagePrepaidExpenses && (
                        <HStack spacing={2}>
                          <Button size="xs" variant="outline" leftIcon={<Icon as={FiEdit2} />} onClick={() => handleEditScheduleClick(sched)}>
                            Edit Jadwal
                          </Button>
                          <Button size="xs" colorScheme="purple" onClick={() => handleAmortize(sched.id)}>
                            Eksekusi Jurnal
                          </Button>
                        </HStack>
                      )}
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>

          </ModalBody>
          <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
             <Button onClick={onClose}>Tutup</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <Modal
        isOpen={isEditScheduleOpen}
        onClose={() => {
          if (isScheduleSubmitting) return;
          onEditScheduleClose();
          setScheduleToEdit(null);
        }}
      >
        <ModalOverlay />
        <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
          <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>
            Edit Jadwal Prepaid
          </ModalHeader>
          <ModalCloseButton color={textColor} isDisabled={isScheduleSubmitting} />
          <ModalBody py={6}>
            <VStack spacing={4} align="stretch">
              <Box bg={panelBg} borderWidth="1px" borderColor={borderColor} borderRadius="md" p={4}>
                <Text fontWeight="semibold" color={textColor}>
                  {selectedPrepaid?.name}
                </Text>
                <Text fontSize="sm" color={mutedTextColor}>
                  Jadwal bulan ke-{scheduleToEdit?.month_number}
                </Text>
              </Box>

              <FormControl isRequired>
                <FormLabel color={textColor}>Tanggal Jadwal Baru</FormLabel>
                <Input
                  type="date"
                  value={editScheduledDate}
                  onChange={(e) => setEditScheduledDate(e.target.value)}
                  bg={inputBg}
                  borderColor={inputBorderColor}
                  color={textColor}
                  _hover={{ borderColor: inputBorderColor }}
                  _focusVisible={fieldFocusStyles}
                />
              </FormControl>

              <Text fontSize="sm" color={mutedTextColor}>
                Hanya jadwal yang masih pending yang bisa diubah. Saat tanggal ini disimpan, semua jadwal setelahnya akan ikut menyesuaikan otomatis per bulan.
              </Text>
            </VStack>
          </ModalBody>
          <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
            <Button
              variant="ghost"
              mr={3}
              onClick={() => {
                onEditScheduleClose();
                setScheduleToEdit(null);
              }}
              isDisabled={isScheduleSubmitting}
            >
              Batal
            </Button>
            <Button colorScheme="blue" onClick={handleUpdateScheduleSubmit} isLoading={isScheduleSubmitting}>
              Simpan Jadwal
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Modal Terminate */}
      <Modal isOpen={isTerminateOpen} onClose={onTerminateClose}>
        <ModalOverlay />
        <ModalContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
          <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={borderColor}>Terminasi Prepaid - {prepaidToTerminate?.name}</ModalHeader>
          <ModalCloseButton color={textColor} />
          <ModalBody>
            <VStack spacing={4} align="stretch">
              <Box>
                 <Text fontWeight="bold" mb={2} color={textColor}>Tipe Terminasi</Text>
                 <Select value={terminateType} onChange={(e) => setTerminateType(e.target.value)} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles}>
                   <option value="REFUND">Refund (Pengembalian Dana)</option>
                   <option value="WRITEOFF">Write-off (Penghapusan/Pembebanan Sekaligus)</option>
                 </Select>
                 <Text fontSize="sm" color={mutedTextColor} mt={1}>
                   {terminateType === 'REFUND'
                     ? 'Pilih Refund jika sisa dana dikembalikan oleh vendor (masuk ke Kas/Bank).'
                     : 'Pilih Write-off jika sisa dana tidak dikembalikan dan diakui sebagai beban sisa sekaligus.'}
                 </Text>
              </Box>

              <Box>
                 <Text fontWeight="bold" mb={2} color={textColor}>
                   {terminateType === 'REFUND' ? 'Akun Penerimaan (Kas/Bank)' : 'Akun Beban / Kerugian'}
                 </Text>
                 <Select value={destinationAccount} onChange={(e) => setDestinationAccount(e.target.value)} bg={inputBg} borderColor={inputBorderColor} color={textColor} _hover={{ borderColor: inputBorderColor }} _focusVisible={fieldFocusStyles}>
                   <option value="">-- Pilih Akun --</option>
                   {accounts
                     .filter((a: Account) => {
                       if (a.is_header) return false;
                       if (terminateType === 'REFUND') {
                         return (a.type === 'ASSET' && a.category === 'CASH_BANK') || a.code.startsWith('1101');
                       } else {
                         return a.type === 'EXPENSE' || a.type === 'OTHER_EXPENSE';
                       }
                     })
                     .map(acc => (
                       <option key={acc.id} value={acc.id}>{acc.code} - {acc.name}</option>
                     ))}
                 </Select>
              </Box>
            </VStack>
          </ModalBody>
          <ModalFooter bg={modalFooterBg} borderTopWidth="1px" borderColor={borderColor}>
             <Button variant="ghost" mr={3} onClick={onTerminateClose}>Batal</Button>
             <Button colorScheme="red" onClick={handleTerminateSubmit}>Proses Terminasi</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <AlertDialog
        isOpen={isDeleteOpen}
        leastDestructiveRef={cancelDeleteRef}
        onClose={() => {
          if (isDeleteSubmitting) return;
          onDeleteClose();
          setPrepaidToDelete(null);
        }}
      >
        <AlertDialogOverlay>
          <AlertDialogContent bg={modalBg} borderWidth="1px" borderColor={borderColor}>
            <AlertDialogHeader fontSize="lg" fontWeight="bold" color={textColor}>
              Hapus Transaksi Prepaid
            </AlertDialogHeader>

            <AlertDialogBody>
              <Text mb={2} color={textColor}>
                Hapus prepaid <strong>{prepaidToDelete?.name}</strong>?
              </Text>
              <Text fontSize="sm" color={mutedTextColor}>
                Aksi ini hanya untuk admin dan akan gagal jika prepaid sudah memiliki jurnal amortisasi atau terminasi.
              </Text>
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelDeleteRef} onClick={() => {
                onDeleteClose();
                setPrepaidToDelete(null);
              }} isDisabled={isDeleteSubmitting}>
                Batal
              </Button>
              <Button colorScheme="red" onClick={handleDeleteSubmit} ml={3} isLoading={isDeleteSubmitting}>
                Hapus
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>

    </Box>
    </SimpleLayout>
  );
}
