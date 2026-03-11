'use client';

import { useState, useEffect } from 'react';
import SimpleLayout from '@/components/layout/SimpleLayout';
import {
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
  VStack,
  HStack,
  Divider,
  Select
} from '@chakra-ui/react';
import { FiRefreshCw, FiEye, FiCheckCircle, FiXCircle } from 'react-icons/fi';
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
  const [prepaids, setPrepaids] = useState<PrepaidExpense[]>([]);
  const [schedules, setSchedules] = useState<PrepaidSchedule[]>([]);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [selectedPrepaid, setSelectedPrepaid] = useState<PrepaidExpense | null>(null);
  const [loading, setLoading] = useState(false);

  const { isOpen, onOpen, onClose } = useDisclosure();
  const { isOpen: isTerminateOpen, onOpen: onTerminateOpen, onClose: onTerminateClose } = useDisclosure();
  const [terminateType, setTerminateType] = useState('REFUND');
  const [destinationAccount, setDestinationAccount] = useState('');
  const [prepaidToTerminate, setPrepaidToTerminate] = useState<PrepaidExpense | null>(null);
  const toast = useToast();

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
      setSchedules(response.data.data.schedules || []);
      onOpen();
    } catch (error: any) {
      toast({
        title: 'Error',
        description: 'Gagal memuat jadwal amortisasi',
        status: 'error',
        duration: 3000
      });
    }
  };

  const handleAmortize = async (scheduleId: number) => {
    if (!selectedPrepaid) return;

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
    setPrepaidToTerminate(prepaid);
    setTerminateType('REFUND');
    setDestinationAccount('');
    onTerminateOpen();
  };

  const handleTerminateSubmit = async () => {
    if (!prepaidToTerminate) return;

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

  useEffect(() => {
    fetchPrepaids();
    fetchAccounts();
  }, []);

  const formatCurrency = (val: number) => new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(val);
  const formatDate = (dateString: string) => new Date(dateString).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });

  return (
    <SimpleLayout title="Prepaid Expenses">
      <Box p={6}>
      <Flex justifyContent="space-between" alignItems="center" mb={6}>
        <Heading size="lg">Prepaid Expenses (Amortisasi)</Heading>
        <Button leftIcon={<Icon as={FiRefreshCw} />} onClick={fetchPrepaids} isLoading={loading}>
          Refresh
        </Button>
      </Flex>

      <Box bg="white" p={4} borderRadius="md" shadow="sm" overflowX="auto">
        <Table variant="simple" size="sm">
          <Thead bg="gray.50">
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
              <Tr key={item.id}>
                <Td fontWeight="bold">{item.name}</Td>
                <Td>PO #{item.purchase_id}</Td>
                <Td isNumeric>{formatCurrency(item.total_amount)}</Td>
                <Td isNumeric color="purple.600" fontWeight="bold">
                  {formatCurrency(item.monthly_amount)}
                </Td>
                <Td>
                  {item.amortization_month} Bulan <br/>
                  <Text fontSize="xs" color="gray.500">
                    ({formatDate(item.start_date)} - {formatDate(item.end_date)})
                  </Text>
                </Td>
                <Td>
                  <Badge colorScheme={item.status === 'ACTIVE' ? 'purple' : item.status === 'COMPLETED' ? 'green' : 'red'}>
                    {item.status}
                  </Badge>
                </Td>
                <Td>
                  <Button size="sm" colorScheme="blue" leftIcon={<Icon as={FiEye} />} onClick={() => loadPrepaidDetails(item)}>
                    Jadwal
                  </Button>
                  {item.status === 'ACTIVE' && (
                    <Button ml={2} size="sm" colorScheme="orange" leftIcon={<Icon as={FiXCircle} />} onClick={() => handleTerminateClick(item)}>
                      Terminasi
                    </Button>
                  )}
                </Td>
              </Tr>
            ))}
            {prepaids.length === 0 && !loading && (
              <Tr><Td colSpan={7} textAlign="center" py={6} color="gray.500">Tidak ada data prepaid expense</Td></Tr>
            )}
          </Tbody>
        </Table>
      </Box>

      {/* Modal Detail & Jadwal */}
      <Modal isOpen={isOpen} onClose={onClose} size="4xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Jadwal Amortisasi - {selectedPrepaid?.name}</ModalHeader>
          <ModalCloseButton />

          <ModalBody>
            <Flex gap={6} mb={6} bg="gray.50" p={4} borderRadius="md">
              <VStack align="start" flex={1} spacing={2}>
                <Text fontWeight="bold" color="gray.500" fontSize="sm">Total Saldo Prepaid</Text>
                <Text fontSize="xl" fontWeight="bold">{selectedPrepaid && formatCurrency(selectedPrepaid.total_amount)}</Text>
              </VStack>
              <VStack align="start" flex={1} spacing={2}>
                <Text fontWeight="bold" color="gray.500" fontSize="sm">Akun Beban Tujuan (Expense)</Text>
                <Select size="sm" bg="white"
                  value={selectedPrepaid?.expense_account_id || ''}
                  onChange={handleSetExpenseAccount}
                >
                  <option value="">-- Pilih Akun Beban --</option>
                  {accounts
                    .filter((a: Account) => !a.is_header && (a.type === 'EXPENSE' || a.type === 'OTHER_EXPENSE'))
                    .map(acc => (
                      <option key={acc.id} value={acc.id}>{acc.code} - {acc.name}</option>
                    ))}
                </Select>
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
                      {sched.status === 'PENDING' && (
                        <Button size="xs" colorScheme="purple" onClick={() => handleAmortize(sched.id)}>
                          Eksekusi Jurnal
                        </Button>
                      )}
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>

          </ModalBody>
          <ModalFooter>
             <Button onClick={onClose}>Tutup</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Modal Terminate */}
      <Modal isOpen={isTerminateOpen} onClose={onTerminateClose}>
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Terminasi Prepaid - {prepaidToTerminate?.name}</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} align="stretch">
              <Box>
                 <Text fontWeight="bold" mb={2}>Tipe Terminasi</Text>
                 <Select value={terminateType} onChange={(e) => setTerminateType(e.target.value)}>
                   <option value="REFUND">Refund (Pengembalian Dana)</option>
                   <option value="WRITEOFF">Write-off (Penghapusan/Pembebanan Sekaligus)</option>
                 </Select>
                 <Text fontSize="sm" color="gray.500" mt={1}>
                   {terminateType === 'REFUND'
                     ? 'Pilih Refund jika sisa dana dikembalikan oleh vendor (masuk ke Kas/Bank).'
                     : 'Pilih Write-off jika sisa dana tidak dikembalikan dan diakui sebagai beban sisa sekaligus.'}
                 </Text>
              </Box>

              <Box>
                 <Text fontWeight="bold" mb={2}>
                   {terminateType === 'REFUND' ? 'Akun Penerimaan (Kas/Bank)' : 'Akun Beban / Kerugian'}
                 </Text>
                 <Select value={destinationAccount} onChange={(e) => setDestinationAccount(e.target.value)}>
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
          <ModalFooter>
             <Button variant="ghost" mr={3} onClick={onTerminateClose}>Batal</Button>
             <Button colorScheme="red" onClick={handleTerminateSubmit}>Proses Terminasi</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

    </Box>
    </SimpleLayout>
  );
}
