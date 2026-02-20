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
import cashbankService, { CashBank } from '@/services/cashbankService';

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

  // Helper to format currency without decimals
  const formatRupiah = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0,
    }).format(amount);
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
      setCashBanks(accounts || []);
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

        // ⚠️ IMPORTANT: Use Math.abs() for both values as a defensive measure.
        // PPN Masukan (2102-002) is a LIABILITY account, so its raw balance
        // may be negative. Backend now returns absolute values, but we guard
        // against any future regression. This ensures:
        //   CORRECT: 6,600,000 - 1,100,000 = 5,500,000
        //   NOT:     6,600,000 - (-1,100,000) = 7,700,000
        const ppnMasukan = Math.abs(masukanData.balance || 0);
        const ppnKeluaran = Math.abs(keluaranData.balance || 0);
        // Round to avoid floating point precision issues
        const ppnTerutang = Math.round(ppnKeluaran - ppnMasukan);

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

              {/* PPN Already Paid / No Outstanding */}
              {!loadingBalance && ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang === 0 && (
                <Alert status="success" borderRadius="md">
                  <AlertIcon />
                  <Box fontSize="sm">
                    <Text fontWeight="bold">✅ PPN Sudah Lunas</Text>
                    <Text mt={1}>
                      Tidak ada PPN yang harus dibayar saat ini. Semua kewajiban PPN sudah diselesaikan.
                    </Text>
                  </Box>
                </Alert>
              )}

              {/* PPN Calculation Card */}
              {!loadingBalance && ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang > 0 && (
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

                      {/* Result */}
                      <Box
                        borderTop="2px"
                        borderColor={ppnBalanceInfo.ppn_terutang > 0 ? 'red.400' : 'green.400'}
                        pt={3}
                        bg={ppnBalanceInfo.ppn_terutang > 0 ? resultBgKurangBayar : resultBgLebihBayar}
                        p={3}
                        borderRadius="md"
                      >
                        <HStack justify="space-between" mb={2}>
                          <Text fontSize="lg" fontWeight="bold" color={ppnBalanceInfo.ppn_terutang > 0 ? 'red.600' : 'green.600'}>
                            PPN Terutang:
                          </Text>
                          <Text fontSize="xl" fontWeight="bold" color={ppnBalanceInfo.ppn_terutang > 0 ? 'red.600' : 'green.600'}>
                            {formatRupiah(Math.abs(ppnBalanceInfo.ppn_terutang))}
                          </Text>
                        </HStack>
                        <Badge
                          colorScheme={ppnBalanceInfo.ppn_terutang > 0 ? 'red' : 'green'}
                          fontSize="xs"
                          px={2}
                          py={1}
                        >
                          {ppnBalanceInfo.ppn_terutang > 0
                            ? '⚠️ KURANG BAYAR - Harus dibayar ke negara'
                            : '✅ LEBIH BAYAR - Bisa dikompensasi periode berikutnya'
                          }
                        </Badge>

                        {ppnBalanceInfo.ppn_terutang > 0 && (
                          <Text fontSize="xs" color={mutedColor} mt={2} fontStyle="italic">
                            * Jumlah ini yang harus disetor ke negara melalui e-Billing DJP
                          </Text>
                        )}
                      </Box>
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

              {/* Amount (Read-only - always full PPN Terutang) */}
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
                  bg={useColorModeValue('blue.50', 'blue.900')}
                  borderRadius="md"
                  border="1px solid"
                  borderColor={useColorModeValue('blue.200', 'blue.700')}
                >
                  <Text fontSize="lg" fontWeight="bold" color={useColorModeValue('blue.700', 'blue.200')}>
                    {ppnBalanceInfo && ppnBalanceInfo.ppn_terutang > 0
                      ? formatRupiah(ppnBalanceInfo.ppn_terutang)
                      : 'Rp 0'}
                  </Text>
                  <Text fontSize="xs" color={mutedColor} mt={1}>
                    Pembayaran penuh PPN Terutang (tidak bisa cicilan)
                  </Text>
                </Box>
              </FormControl>

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
                      {cashBanks.map((cashBank) => (
                        <option key={cashBank.id} value={cashBank.id}>
                          {cashBank.name} - Saldo: {formatRupiah(cashBank.balance)}
                        </option>
                      ))}
                    </Select>
                  )}
                />
                <FormErrorMessage>{errors.cash_bank_id?.message}</FormErrorMessage>
              </FormControl>

              {/* Balance Validation */}
              {ppnBalanceInfo && ppnBalanceInfo.ppn_terutang > 0 && selectedCashBankId > 0 && (() => {
                const selectedCB = getSelectedCashBank();
                if (selectedCB && ppnBalanceInfo.ppn_terutang > selectedCB.balance) {
                  return (
                    <Alert status="warning" borderRadius="md">
                      <AlertIcon />
                      <Text fontSize="sm">
                        Saldo kas/bank tidak mencukupi. Saldo: {formatRupiah(selectedCB.balance)}
                      </Text>
                    </Alert>
                  );
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
              {ppnBalanceInfo && ppnBalanceInfo.ppn_terutang > 0 && (
                <Card bg={summaryBg} borderColor={borderColor}>
                  <CardBody>
                    <VStack align="stretch" spacing={2}>
                      <Text fontWeight="bold" fontSize="sm">Ringkasan:</Text>
                      <HStack justify="space-between">
                        <Text fontSize="sm">Jumlah Pembayaran:</Text>
                        <Text fontSize="sm" fontWeight="bold">
                          {formatRupiah(ppnBalanceInfo.ppn_terutang)}
                        </Text>
                      </HStack>
                      <HStack justify="space-between">
                        <Text fontSize="sm">PPN Keluaran:</Text>
                        <Badge colorScheme="red">Berkurang (Debit)</Badge>
                      </HStack>
                      <HStack justify="space-between">
                        <Text fontSize="sm">PPN Masukan:</Text>
                        <Badge colorScheme="green">Dikompensasi (Credit)</Badge>
                      </HStack>
                      <HStack justify="space-between">
                        <Text fontSize="sm">Kas/Bank:</Text>
                        <Badge colorScheme="red">Keluar (Credit)</Badge>
                      </HStack>
                    </VStack>
                  </CardBody>
                </Card>
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
              isDisabled={ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang === 0}
            >
              {ppnBalanceInfo !== null && ppnBalanceInfo.ppn_terutang === 0
                ? '✅ Sudah Lunas'
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
