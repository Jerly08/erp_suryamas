'use client';

import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  VStack,
  HStack,
  FormControl,
  FormLabel,
  FormErrorMessage,
  Input,
  Select,
  Textarea,
  Box,
  Text,
  useToast,
  Alert,
  AlertIcon,
  Switch,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Card,
  CardBody,
  SimpleGrid,
  Badge,
  Tooltip,
  Icon,
  useColorModeValue,
} from '@chakra-ui/react';
import { useForm, Controller } from 'react-hook-form';
import { FiInfo, FiDollarSign } from 'react-icons/fi';
import { useAuth } from '@/contexts/AuthContext';
import cashbankService, { CashBank, getBookBalanceInIDR } from '@/services/cashbankService';
import { filterOperationalCashBankAccounts } from '@/utils/accountFilters';

interface PPNPaymentModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
  ppnType: 'INPUT' | 'OUTPUT'; // INPUT for PPN Masukan, OUTPUT for PPN Keluaran
}

interface PPNPaymentFormData {
  ppn_type: 'INPUT' | 'OUTPUT';
  amount: number;
  date: string;
  cash_bank_id: number;
  reference: string;
  notes: string;
}

interface PPNBalanceInfo {
  ppn_masukan: number;
  ppn_keluaran: number;
  ppn_terutang: number;
}

const PPNPaymentModal: React.FC<PPNPaymentModalProps> = ({
  isOpen,
  onClose,
  onSuccess,
  ppnType,
}) => {
  const { token } = useAuth();
  const toast = useToast();

  // Helper to format currency with decimals when present
  const formatRupiah = (amount: number) => {
    const hasFraction = Math.abs((amount || 0) % 1) > Number.EPSILON;

    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: hasFraction ? 2 : 0,
      maximumFractionDigits: 2,
    }).format(amount);
  };

  const formatCashBankBalance = (amount: number, currency?: string) => {
    const normalizedCurrency = (currency || 'IDR').toUpperCase();

    if (normalizedCurrency === 'USD') {
      return `USD ${(amount || 0).toLocaleString('en-US', {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
      })}`;
    }

    return formatRupiah(amount || 0);
  };

  const getAvailableBookBalance = (account: CashBank) => {
    const nativeAvailable = Math.max(0, (account.balance || 0) - (account.frozen_amount || 0));

    if ((account.currency || 'IDR').toUpperCase() === 'IDR') {
      return nativeAvailable;
    }

    if ((account.balance || 0) <= 0) {
      return 0;
    }

    return getBookBalanceInIDR(account) * (nativeAvailable / account.balance);
  };

  const formatCashBankAvailability = (account: CashBank) => {
    const nativeAvailable = Math.max(0, (account.balance || 0) - (account.frozen_amount || 0));
    const nativeLabel = formatCashBankBalance(nativeAvailable, account.currency);

    if ((account.currency || 'IDR').toUpperCase() === 'IDR') {
      return nativeLabel;
    }

    return `${nativeLabel} | Nilai buku: ${formatRupiah(getAvailableBookBalance(account))}`;
  };

  const [loading, setLoading] = useState(false);
  const [cashBanks, setCashBanks] = useState<CashBank[]>([]);
  const [ppnBalanceInfo, setPPNBalanceInfo] = useState<PPNBalanceInfo | null>(null);
  const [loadingBalance, setLoadingBalance] = useState(false);

  // Color mode values - MUST be at top level (Rules of Hooks)
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const mutedColor = useColorModeValue('gray.600', 'gray.400');
  const summaryBg = useColorModeValue('gray.50', 'gray.700');
  const ppnCardBg = useColorModeValue('blue.50', 'blue.900');
  const resultBgKurangBayar = useColorModeValue('red.50', 'red.900');
  const resultBgLebihBayar = useColorModeValue('green.50', 'green.900');
  const lebihBayarBg = useColorModeValue('blue.50', 'blue.900');
  const lebihBayarInfoBg = useColorModeValue('white', 'gray.700');

  const {
    control,
    handleSubmit,
    formState: { errors },
    reset,
    watch,
  } = useForm<PPNPaymentFormData>({
    defaultValues: {
      ppn_type: ppnType,
      amount: 0,
      date: new Date().toISOString().split('T')[0],
      cash_bank_id: 0,
      reference: '',
      notes: '',
    },
  });

  const selectedCashBankId = watch('cash_bank_id');

  // Load cash/bank accounts
  useEffect(() => {
    if (isOpen && token) {
      loadCashBanks();
      loadPPNBalance();
    }
  }, [isOpen, token]);

  const loadCashBanks = async () => {
    try {
      const accounts = await cashbankService.getPaymentAccounts();
      setCashBanks(filterOperationalCashBankAccounts(accounts || []));
    } catch (error: any) {
      console.error('Failed to load cash/bank accounts:', error);
      toast({
        title: 'Error',
        description: 'Gagal memuat akun kas/bank',
        status: 'error',
        duration: 3000,
      });
    }
  };

  const loadPPNBalance = async () => {
    if (!token) return;

    setLoadingBalance(true);
    try {
      console.log('🔍 Loading PPN balance...');
      console.log('🔑 Token:', token ? 'Present' : 'Missing');

      const inputUrl = `/api/v1/tax-payments/ppn/balance?type=INPUT`;
      const outputUrl = `/api/v1/tax-payments/ppn/balance?type=OUTPUT`;

      console.log('🎯 Fetching URLs:', { inputUrl, outputUrl });

      // Load both PPN Masukan and Keluaran balances
      const [masukanRes, keluaranRes] = await Promise.all([
        fetch(inputUrl, {
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        }),
        fetch(outputUrl, {
          headers: {
            'Authorization': `Bearer ${token}`,
            'Content-Type': 'application/json',
          },
        }),
      ]);

      console.log('📊 Response status:', {
        masukan: masukanRes.status,
        masukanOk: masukanRes.ok,
        keluaran: keluaranRes.status,
        keluaranOk: keluaranRes.ok,
      });

      // Try to get error messages if not ok
      if (!masukanRes.ok) {
        const errorText = await masukanRes.text();
        console.error('❌ Masukan API Error:', masukanRes.status, errorText);
      }
      if (!keluaranRes.ok) {
        const errorText = await keluaranRes.text();
        console.error('❌ Keluaran API Error:', keluaranRes.status, errorText);
      }

      if (masukanRes.ok && keluaranRes.ok) {
        const masukanData = await masukanRes.json();
        const keluaranData = await keluaranRes.json();

        console.log('💰 PPN Balance Data:', { masukanData, keluaranData });

        // Use Math.abs() for both values as a defensive measure. PPN Masukan
        // is now an asset account, but older databases may still carry legacy
        // contra-liability balances until they are reclassified. Backend now
        // returns absolute values, but we guard against any future regression:
        //   CORRECT: 6,600,000 - 1,100,000 = 5,500,000
        //   NOT:     6,600,000 - (-1,100,000) = 7,700,000
        const ppnMasukan = Math.abs(masukanData.balance || 0);
        const ppnKeluaran = Math.abs(keluaranData.balance || 0);
        // Round to 2 decimals to avoid floating point precision issues without losing cents.
        const ppnTerutang = Math.round((ppnKeluaran - ppnMasukan) * 100) / 100;

        console.log('✅ Calculated:', { ppnMasukan, ppnKeluaran, ppnTerutang });

        setPPNBalanceInfo({
          ppn_masukan: ppnMasukan,
          ppn_keluaran: ppnKeluaran,
          ppn_terutang: ppnTerutang,
        });
      } else {
        console.error('❌ API Error:', {
          masukanStatus: masukanRes.status,
          keluaranStatus: keluaranRes.status
        });

        // Set default values if API fails
        setPPNBalanceInfo({
          ppn_masukan: 0,
          ppn_keluaran: 0,
          ppn_terutang: 0,
        });

        toast({
          title: 'Warning',
          description: 'Gagal memuat balance PPN. Menggunakan nilai 0.',
          status: 'warning',
          duration: 3000,
        });
      }
    } catch (error: any) {
      console.error('❌ Failed to load PPN balance:', error);

      // Set default values on error
      setPPNBalanceInfo({
        ppn_masukan: 0,
        ppn_keluaran: 0,
        ppn_terutang: 0,
      });

      toast({
        title: 'Error',
        description: 'Gagal memuat balance PPN: ' + error.message,
        status: 'error',
        duration: 3000,
      });
    } finally {
      setLoadingBalance(false);
    }
  };

  const getSelectedCashBank = () => {
    return cashBanks.find(cb => cb.id === selectedCashBankId);
  };

  const onSubmit = async (data: PPNPaymentFormData) => {
    if (!token) {
      toast({
        title: 'Error',
        description: 'Anda harus login terlebih dahulu',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    if (!data.cash_bank_id) {
      toast({
        title: 'Error',
        description: 'Pilih akun kas/bank',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    // Always use full PPN Terutang (no partial/cicilan)
    const fullAmount = ppnBalanceInfo?.ppn_terutang || 0;
    if (fullAmount <= 0) {
      toast({
        title: 'Error',
        description: 'Tidak ada PPN Terutang yang harus dibayar',
        status: 'error',
        duration: 3000,
      });
      return;
    }

    setLoading(true);
    try {
      const response = await fetch('/api/v1/tax-payments/ppn', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${token}`,
        },
        body: JSON.stringify({
          ppn_type: data.ppn_type,
          amount: fullAmount,
          date: new Date(data.date).toISOString(),
          cash_bank_id: data.cash_bank_id,
          reference: data.reference,
          notes: data.notes,
        }),
      });

      if (!response.ok) {
        const errorData = await response.json();

        // Check if it's a validation error (PPN already paid, insufficient balance, etc)
        const errorMsg = errorData.error || 'Gagal membuat pembayaran PPN';
        const isValidationError = errorData.type === 'validation_error' ||
          response.status === 400;

        // Special handling for "PPN sudah lunas"
        if (errorMsg.includes('tidak ada PPN yang harus dibayar') ||
          errorMsg.includes('PPN Terutang: 0')) {
          toast({
            title: '✅ PPN Sudah Lunas',
            description: 'Tidak ada PPN yang harus dibayar saat ini. Semua kewajiban PPN sudah diselesaikan.',
            status: 'success',
            duration: 5000,
          });
          reset();
          onClose();
          return;
        }

        // For other validation errors, show as warning
        if (isValidationError) {
          toast({
            title: 'Validasi Gagal',
            description: errorMsg,
            status: 'warning',
            duration: 5000,
          });
          return;
        }

        // For server errors, throw
        throw new Error(errorMsg);
      }

      const result = await response.json();

      toast({
        title: 'Berhasil',
        description: 'Setor PPN ke negara berhasil diproses',
        status: 'success',
        duration: 3000,
      });

      reset();
      onClose();
      if (onSuccess) {
        onSuccess();
      }
    } catch (error: any) {
      console.error('Error creating PPN payment:', error);
      toast({
        title: 'Error',
        description: error.message || 'Gagal membuat pembayaran PPN',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const ppnLabel = 'Setor PPN';
  const ppnDescription = 'Pembayaran PPN terutang ke negara (kompensasi PPN Keluaran dikurangi PPN Masukan)';

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="xl">
      <ModalOverlay />
      <ModalContent bg={bgColor}>
        <ModalHeader>
          <VStack align="start" spacing={1}>
            <Text>Pembayaran {ppnLabel}</Text>
            <Text fontSize="sm" fontWeight="normal" color={mutedColor}>
              {ppnDescription}
            </Text>
          </VStack>
        </ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit(onSubmit)}>
          <ModalBody>
            <VStack spacing={4} align="stretch">
              {/* Info Alert */}
              <Alert status="info" borderRadius="md">
                <AlertIcon />
                <Box fontSize="sm">
                  <Text fontWeight="bold">Jurnal Otomatis (Sesuai PSAK):</Text>
                  <Text>
                    Debit: PPN Keluaran (liability berkurang)<br />
                    Credit: PPN Masukan (asset/kompensasi)<br />
                    Credit: Kas/Bank (pembayaran neto)
                  </Text>
                </Box>
              </Alert>

              {/* Loading Balance */}
              {loadingBalance && (
                <Alert status="info" borderRadius="md">
                  <AlertIcon />
                  <Text fontSize="sm">Memuat balance PPN...</Text>
                </Alert>
              )}

              {/* PPN Calculation Card - Always show when balance loaded */}
              {!loadingBalance && ppnBalanceInfo !== null && (
                <Card bg={ppnCardBg} borderColor={borderColor} borderWidth="2px">
                  <CardBody>
                    <VStack align="stretch" spacing={4}>
                      {/* Title */}
                      <Text fontSize="md" fontWeight="bold" textAlign="center">
                        Perhitungan PPN
                      </Text>

                      {/* PPN Keluaran */}
                      <Box>
                        <HStack justify="space-between" mb={1}>
                          <Text fontSize="sm" fontWeight="medium">PPN Keluaran (dari Penjualan):</Text>
                          <Text fontSize="sm" fontWeight="bold">{formatRupiah(ppnBalanceInfo.ppn_keluaran)}</Text>
                        </HStack>
                        <Text fontSize="xs" color={mutedColor} pl={2}>
                          PPN yang dipungut dari customer
                        </Text>
                      </Box>

                      {/* Minus Sign */}
                      <HStack justify="center">
                        <Box width="full" borderBottom="1px" borderColor={borderColor} />
                        <Text fontSize="lg" fontWeight="bold" px={3}>−</Text>
                        <Box width="full" borderBottom="1px" borderColor={borderColor} />
                      </HStack>

                      {/* PPN Masukan */}
                      <Box>
                        <HStack justify="space-between" mb={1}>
                          <Text fontSize="sm" fontWeight="medium">PPN Masukan (dari Pembelian):</Text>
                          <Text fontSize="sm" fontWeight="bold">{formatRupiah(ppnBalanceInfo.ppn_masukan)}</Text>
                        </HStack>
                        <Text fontSize="xs" color={mutedColor} pl={2}>
                          PPN yang dibayar ke vendor (kredit pajak)
                        </Text>
                      </Box>

                      {/* Result - Kurang Bayar */}
                      {ppnBalanceInfo.ppn_terutang > 0 && (
                        <Box
                          borderTop="2px"
                          borderColor="red.400"
                          pt={3}
                          bg={resultBgKurangBayar}
                          p={3}
                          borderRadius="md"
                        >
                          <HStack justify="space-between" mb={2}>
                            <Text fontSize="lg" fontWeight="bold" color="red.600">
                              PPN Terutang:
                            </Text>
                            <Text fontSize="xl" fontWeight="bold" color="red.600">
                              {formatRupiah(ppnBalanceInfo.ppn_terutang)}
                            </Text>
                          </HStack>
                          <Badge colorScheme="red" fontSize="xs" px={2} py={1}>
                            ⚠️ KURANG BAYAR - Harus dibayar ke negara
                          </Badge>
                          <Text fontSize="xs" color={mutedColor} mt={2} fontStyle="italic">
                            * Jumlah ini yang harus disetor ke negara melalui e-Billing DJP
                          </Text>
                        </Box>
                      )}

                      {/* Result - Sudah Lunas */}
                      {ppnBalanceInfo.ppn_terutang === 0 && (
                        <Box
                          borderTop="2px"
                          borderColor="green.400"
                          pt={3}
                          bg={resultBgLebihBayar}
                          p={3}
                          borderRadius="md"
                        >
                          <HStack justify="space-between" mb={2}>
                            <Text fontSize="lg" fontWeight="bold" color="green.600">
                              PPN Terutang:
                            </Text>
                            <Text fontSize="xl" fontWeight="bold" color="green.600">
                              Rp 0
                            </Text>
                          </HStack>
                          <Badge colorScheme="green" fontSize="xs" px={2} py={1}>
                            ✅ LUNAS - Tidak ada kewajiban PPN saat ini
                          </Badge>
                          <Text fontSize="xs" color={mutedColor} mt={2}>
                            PPN Keluaran dan PPN Masukan sudah seimbang. Tidak ada pembayaran yang perlu dilakukan.
                          </Text>
                        </Box>
                      )}

                      {/* Result - Lebih Bayar */}
                      {ppnBalanceInfo.ppn_terutang < 0 && (
                        <Box
                          borderTop="2px"
                          borderColor="blue.400"
                          pt={3}
                          bg={lebihBayarBg}
                          p={3}
                          borderRadius="md"
                        >
                          <HStack justify="space-between" mb={2}>
                            <Text fontSize="lg" fontWeight="bold" color="blue.600">
                              PPN Lebih Bayar:
                            </Text>
                            <Text fontSize="xl" fontWeight="bold" color="blue.600">
                              {formatRupiah(Math.abs(ppnBalanceInfo.ppn_terutang))}
                            </Text>
                          </HStack>
                          <Badge colorScheme="blue" fontSize="xs" px={2} py={1}>
                            💡 LEBIH BAYAR - PPN Masukan lebih besar dari PPN Keluaran
                          </Badge>
                          <Box mt={3} p={3} bg={lebihBayarInfoBg} borderRadius="md" border="1px" borderColor={borderColor}>
                            <Text fontSize="sm" fontWeight="bold" mb={2} color="blue.600">
                              ℹ️ Apa artinya PPN Lebih Bayar?
                            </Text>
                            <VStack align="stretch" spacing={2}>
                              <Text fontSize="xs">
                                PPN Masukan (kredit pajak dari pembelian) Anda lebih besar dari PPN Keluaran (PPN yang dipungut dari penjualan).
                                Artinya perusahaan Anda memiliki <strong>kelebihan kredit pajak sebesar {formatRupiah(Math.abs(ppnBalanceInfo.ppn_terutang))}</strong>.
                              </Text>
                              <Text fontSize="sm" fontWeight="bold" mt={1}>
                                Apa yang bisa dilakukan?
                              </Text>
                              <Box pl={3}>
                                <Text fontSize="xs">• <strong>Kompensasi:</strong> Kelebihan PPN akan otomatis dikompensasi ke periode pajak berikutnya. Saldo PPN Masukan akan berkurang seiring penjualan baru.</Text>
                                <Text fontSize="xs">• <strong>Restitusi:</strong> Ajukan permohonan pengembalian (restitusi) ke DJP jika jumlah lebih bayar signifikan.</Text>
                                <Text fontSize="xs">• <strong>Tidak perlu pembayaran:</strong> Tidak ada uang yang harus disetor ke negara saat ini.</Text>
                              </Box>
                            </VStack>
                          </Box>
                        </Box>
                      )}
                    </VStack>
                  </CardBody>
                </Card>
              )}

              {/* Payment Date */}
              <FormControl isInvalid={!!errors.date}>
                <FormLabel>
                  Tanggal Pembayaran
                  <Tooltip label="Tanggal saat PPN dibayarkan ke negara">
                    <span>
                      <Icon as={FiInfo} ml={2} color={mutedColor} />
                    </span>
                  </Tooltip>
                </FormLabel>
                <Controller
                  name="date"
                  control={control}
                  rules={{ required: 'Tanggal pembayaran wajib diisi' }}
                  render={({ field }) => (
                    <Input type="date" {...field} />
                  )}
                />
                <FormErrorMessage>{errors.date?.message}</FormErrorMessage>
              </FormControl>

              {/* Amount (Read-only - only show when there's PPN to pay) */}
              {ppnBalanceInfo && ppnBalanceInfo.ppn_terutang > 0 && (
                <FormControl>
                  <FormLabel>
                    Jumlah Pembayaran
                    <Tooltip label="Pembayaran PPN selalu menggunakan jumlah penuh PPN Terutang">
                      <span>
                        <Icon as={FiInfo} ml={2} color={mutedColor} />
                      </span>
                    </Tooltip>
                  </FormLabel>
                  <Box
                    p={3}
                    bg={lebihBayarBg}
                    borderRadius="md"
                    border="1px solid"
                    borderColor={borderColor}
                  >
                    <Text fontSize="lg" fontWeight="bold" color="blue.600">
                      {formatRupiah(ppnBalanceInfo.ppn_terutang)}
                    </Text>
                    <Text fontSize="xs" color={mutedColor} mt={1}>
                      Pembayaran penuh PPN Terutang (tidak bisa cicilan)
                    </Text>
                  </Box>
                </FormControl>
              )}

              {/* Payment details - only show when PPN needs to be paid */}
              {ppnBalanceInfo && ppnBalanceInfo.ppn_terutang > 0 && (
                <>
                  {/* Cash/Bank Account */}
                  <FormControl isInvalid={!!errors.cash_bank_id}>
                    <FormLabel>
                      Akun Kas/Bank
                      <Tooltip label="Pilih akun kas/bank yang digunakan untuk membayar PPN">
                        <span>
                          <Icon as={FiInfo} ml={2} color={mutedColor} />
                        </span>
                      </Tooltip>
                    </FormLabel>
                    <Controller
                      name="cash_bank_id"
                      control={control}
                      rules={{ required: 'Akun kas/bank wajib dipilih' }}
                      render={({ field }) => (
                        <Select
                          {...field}
                          placeholder="Pilih akun kas/bank"
                          onChange={(e) => field.onChange(parseInt(e.target.value))}
                        >
                          {cashBanks.map((cashBank) => {
                            const frozen = cashBank.frozen_amount || 0;
                            return (
                            <option key={cashBank.id} value={cashBank.id}>
                              {cashBank.name} - Tersedia: {formatCashBankAvailability(cashBank)}{frozen > 0 ? ` [Terblokir: ${formatCashBankBalance(frozen, cashBank.currency)}]` : ''}
                            </option>
                            );
                          })}
                        </Select>
                      )}
                    />
                    <FormErrorMessage>{errors.cash_bank_id?.message}</FormErrorMessage>
                  </FormControl>

                  {/* Balance Validation */}
                  {selectedCashBankId > 0 && (() => {
                    const selectedCB = getSelectedCashBank();
                    if (selectedCB) {
                      const frozen = selectedCB.frozen_amount || 0;
                      const availableBookBalance = getAvailableBookBalance(selectedCB);
                      if (ppnBalanceInfo.ppn_terutang > availableBookBalance) {
                      return (
                        <Alert status="warning" borderRadius="md">
                          <AlertIcon />
                          <Text fontSize="sm">
                            Saldo tersedia tidak mencukupi. Tersedia: {formatCashBankAvailability(selectedCB)}{frozen > 0 ? ` (Total: ${formatCashBankBalance(selectedCB.balance, selectedCB.currency)}, Terblokir: ${formatCashBankBalance(frozen, selectedCB.currency)})` : ''}
                          </Text>
                        </Alert>
                      );
                      }
                    }
                    return null;
                  })()}

                  {/* Reference */}
                  <FormControl>
                    <FormLabel>
                      Nomor Referensi (Opsional)
                      <Tooltip label="Nomor referensi pembayaran seperti nomor SSP atau kode billing">
                        <span>
                          <Icon as={FiInfo} ml={2} color={mutedColor} />
                        </span>
                      </Tooltip>
                    </FormLabel>
                    <Controller
                      name="reference"
                      control={control}
                      render={({ field }) => (
                        <Input
                          {...field}
                          placeholder="Contoh: SSP-123456 atau kode billing"
                        />
                      )}
                    />
                  </FormControl>

                  {/* Notes */}
                  <FormControl>
                    <FormLabel>
                      Catatan (Opsional)
                      <Tooltip label="Catatan tambahan untuk pembayaran PPN ini">
                        <span>
                          <Icon as={FiInfo} ml={2} color={mutedColor} />
                        </span>
                      </Tooltip>
                    </FormLabel>
                    <Controller
                      name="notes"
                      control={control}
                      render={({ field }) => (
                        <Textarea
                          {...field}
                          placeholder="Catatan tambahan..."
                          rows={3}
                        />
                      )}
                    />
                  </FormControl>

                  {/* Summary */}
                  <Card bg={summaryBg} borderColor={borderColor}>
                    <CardBody>
                      <VStack align="stretch" spacing={2}>
                        <Text fontWeight="bold" fontSize="sm">Ringkasan Jurnal:</Text>
                        <HStack justify="space-between">
                          <Text fontSize="sm">Jumlah Pembayaran:</Text>
                          <Text fontSize="sm" fontWeight="bold">
                            {formatRupiah(ppnBalanceInfo.ppn_terutang)}
                          </Text>
                        </HStack>
                        <HStack justify="space-between">
                          <Text fontSize="sm">PPN Keluaran:</Text>
                          <Badge colorScheme="red">Berkurang (Debit) {formatRupiah(ppnBalanceInfo.ppn_keluaran)}</Badge>
                        </HStack>
                        <HStack justify="space-between">
                          <Text fontSize="sm">PPN Masukan:</Text>
                          <Badge colorScheme="green">Dikompensasi (Credit) {formatRupiah(ppnBalanceInfo.ppn_masukan)}</Badge>
                        </HStack>
                        <HStack justify="space-between">
                          <Text fontSize="sm">Kas/Bank:</Text>
                          <Badge colorScheme="red">Keluar (Credit) {formatRupiah(ppnBalanceInfo.ppn_terutang)}</Badge>
                        </HStack>
                      </VStack>
                    </CardBody>
                  </Card>
                </>
              )}
            </VStack>
          </ModalBody>

          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={handleClose}>
              Batal
            </Button>
            <Button
              colorScheme="blue"
              type="submit"
              isLoading={loading}
              loadingText="Memproses..."
              isDisabled={ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang <= 0}
            >
              {ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang === 0
                ? '✅ Sudah Lunas'
                : ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang < 0
                  ? '💡 Tidak Perlu Bayar (Lebih Bayar)'
                  : 'Bayar PPN'
              }
            </Button>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default PPNPaymentModal;
