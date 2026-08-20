'use client';

import React, { useEffect, useState } from 'react';
import {
  Alert,
  AlertDescription,
  AlertIcon,
  AlertTitle,
  Badge,
  Box,
  Button,
  Divider,
  Flex,
  FormControl,
  FormLabel,
  HStack,
  Input,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  NumberDecrementStepper,
  NumberIncrementStepper,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  Select,
  SimpleGrid,
  Stat,
  StatHelpText,
  StatLabel,
  StatNumber,
  Text,
  Textarea,
  useColorModeValue,
  useToast,
  VStack,
} from '@chakra-ui/react';
import cashbankService, {
  CashBank,
  CashBankCounterpartyAccount,
  WithdrawalRequest,
  getAvailableBalance,
  getBookBalanceInIDR,
} from '@/services/cashbankService';
import { useAuth } from '@/contexts/AuthContext';
import { isFinanceManagerRole, normalizeRole } from '@/utils/roles';
import { useTranslation } from '@/hooks/useTranslation';
import {
  getCashBankAmountPrecision,
  isCashBankForeignCurrency,
  normalizeCashBankCurrency,
} from '@/utils/cashBankCurrency';

type WithdrawalPurpose =
  | 'NON_OPERATING_EXPENSE'
  | 'OPERATING_EXPENSE'
  | 'LIABILITY_PAYMENT'
  | 'ASSET_ACQUISITION'
  | 'OWNER_DRAWING';

interface WithdrawalTargetAccounts {
  operatingExpense: CashBankCounterpartyAccount[];
  nonOperatingExpense: CashBankCounterpartyAccount[];
  liability: CashBankCounterpartyAccount[];
  asset: CashBankCounterpartyAccount[];
  ownerDrawing: CashBankCounterpartyAccount[];
}

interface DepositWithdrawalFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account: CashBank | null;
  mode: 'withdrawal';
}

interface ApiErrorShape {
  response?: {
    data?: {
      details?: string;
      message?: string;
      error?: string;
    };
  };
  code?: string;
  message?: string;
}

const withdrawalPurposeOptions: Array<{
  value: WithdrawalPurpose;
  label: string;
  helper: string;
  posting: string;
}> = [
  {
    value: 'NON_OPERATING_EXPENSE',
    label: 'Beban Non Operasional',
    helper: 'Dipakai untuk bunga, rugi selisih kurs, rugi penjualan aset, dan beban non-operasional lain.',
    posting: 'Dr 8xxx / Cr Cash-Bank',
  },
  {
    value: 'OPERATING_EXPENSE',
    label: 'Beban Operasional',
    helper: 'Dipakai untuk biaya operasional rutin seperti sewa, listrik, utilitas, dan biaya administrasi.',
    posting: 'Dr 6xxx / Cr Cash-Bank',
  },
  {
    value: 'LIABILITY_PAYMENT',
    label: 'Pembayaran Liability',
    helper: 'Dipakai untuk pelunasan utang atau pengurangan kewajiban dari kas/bank.',
    posting: 'Dr Liability / Cr Cash-Bank',
  },
  {
    value: 'ASSET_ACQUISITION',
    label: 'Pembelian Asset',
    helper: 'Dipakai untuk pembelian aset yang langsung dibayar dari cash-bank.',
    posting: 'Dr Asset / Cr Cash-Bank',
  },
  {
    value: 'OWNER_DRAWING',
    label: 'Pengambilan Pemilik / Prive',
    helper: 'Dipakai khusus untuk pengambilan pemilik yang harus dicatat ke akun Prive/Owner Drawing, bukan sebagai beban atau pembayaran COA umum.',
    posting: 'Dr Prive / Cr Cash-Bank',
  },
];

const getCurrencyCode = (account?: CashBank | null): string => normalizeCashBankCurrency(account?.currency);

const formatNativeAmount = (currency: string, amount: number): string => `${currency} ${Math.abs(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: getCashBankAmountPrecision(currency),
  maximumFractionDigits: getCashBankAmountPrecision(currency),
})}`;

const formatIDRAmount = (amount: number): string => `IDR ${(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
})}`;

const DepositWithdrawalForm: React.FC<DepositWithdrawalFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  account,
}) => {
  const { user } = useAuth();
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    reference: '',
    notes: '',
    purpose: 'NON_OPERATING_EXPENSE' as WithdrawalPurpose,
    target_account_id: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const [targetAccounts, setTargetAccounts] = useState<WithdrawalTargetAccounts>({
    operatingExpense: [],
    nonOperatingExpense: [],
    liability: [],
    asset: [],
    ownerDrawing: [],
  });

  const toast = useToast();
  const { t } = useTranslation();
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalFooterBg = useColorModeValue('gray.50', 'gray.900');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const sectionTitleColor = useColorModeValue('gray.700', 'gray.100');
  const bodyTextColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.600', 'whiteAlpha.800');
  const helperTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const panelBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const previewBg = useColorModeValue('orange.50', 'rgba(237, 137, 54, 0.12)');
  const previewBorderColor = useColorModeValue('orange.200', 'orange.700');
  const fieldBg = useColorModeValue('white', 'whiteAlpha.50');
  const fieldBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const fieldHoverBorderColor = useColorModeValue('gray.400', 'whiteAlpha.400');
  const fieldFocusBorderColor = useColorModeValue('orange.500', 'orange.300');
  const fieldFocusShadow = `0 0 0 1px ${fieldFocusBorderColor}`;
  const placeholderColor = useColorModeValue('gray.500', 'whiteAlpha.500');
  const fieldStyles = {
    bg: fieldBg,
    color: bodyTextColor,
    borderColor: fieldBorderColor,
    _hover: { borderColor: fieldHoverBorderColor },
    _focusVisible: {
      borderColor: fieldFocusBorderColor,
      boxShadow: fieldFocusShadow,
    },
    _placeholder: { color: placeholderColor },
  };

  useEffect(() => {
    if (!isOpen) {
      return;
    }

    let cancelled = false;

    const loadWithdrawalTargets = async () => {
      try {
        setLoadingAccounts(true);
        const accounts = await cashbankService.getWithdrawalTargetAccounts();
        if (!cancelled) {
          setTargetAccounts(accounts);
        }
      } catch (loadError) {
        console.error('Error loading withdrawal target accounts:', loadError);
        if (!cancelled) {
          setTargetAccounts({
            operatingExpense: [],
            nonOperatingExpense: [],
            liability: [],
            asset: [],
            ownerDrawing: [],
          });
        }
      } finally {
        if (!cancelled) {
          setLoadingAccounts(false);
        }
      }
    };

    void loadWithdrawalTargets();

    return () => {
      cancelled = true;
    };
  }, [isOpen]);

  const handleInputChange = (field: string, value: string | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
    setError(null);
  };

  const handlePurposeChange = (value: WithdrawalPurpose) => {
    setFormData(prev => ({
      ...prev,
      purpose: value,
      target_account_id: '',
    }));
    setError(null);
  };

  const getSelectedTargetAccounts = (): CashBankCounterpartyAccount[] => {
    switch (formData.purpose) {
      case 'OPERATING_EXPENSE':
        return targetAccounts.operatingExpense;
      case 'LIABILITY_PAYMENT':
        return targetAccounts.liability;
      case 'ASSET_ACQUISITION':
        return targetAccounts.asset;
      case 'OWNER_DRAWING':
        return targetAccounts.ownerDrawing;
      case 'NON_OPERATING_EXPENSE':
      default:
        return targetAccounts.nonOperatingExpense;
    }
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      if (!account) {
        throw new Error('No account selected');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      const availableBalance = getAvailableBalance(account);
      if (formData.amount > availableBalance) {
        throw new Error(`Insufficient balance. Available: ${formatNativeAmount(getCurrencyCode(account), availableBalance)}`);
      }

      if (!formData.target_account_id) {
        throw new Error('Pilih akun lawan jurnal terlebih dahulu agar withdrawal diposting ke kelompok yang benar.');
      }

      const isOwnerDrawing = formData.purpose === 'OWNER_DRAWING';
      const normalizedRole = normalizeRole(user?.role);
      const canPostOwnerDrawing = isFinanceManagerRole(normalizedRole) || normalizedRole === 'director';
      if (isOwnerDrawing && !canPostOwnerDrawing) {
        throw new Error('Pengambilan Pemilik / Prive hanya dapat diposting oleh Finance Manager atau Director.');
      }
      if (isOwnerDrawing && !formData.reference.trim()) {
        throw new Error('Nomor referensi/bukti wajib diisi untuk Pengambilan Pemilik / Prive.');
      }
      if (isOwnerDrawing && !formData.notes.trim()) {
        throw new Error('Catatan alasan pengambilan wajib diisi untuk Pengambilan Pemilik / Prive.');
      }

      const requestData: WithdrawalRequest = {
        account_id: account.id,
        date: formData.date,
        amount: formData.amount,
        reference: formData.reference,
        notes: formData.notes,
        target_account_id: parseInt(formData.target_account_id, 10),
        ...(isOwnerDrawing ? { purpose: 'OWNER_DRAWING' as const } : {}),
      };

      await cashbankService.processWithdrawal(requestData);

      const selectedTargetAccount = getSelectedTargetAccounts().find(
        entry => entry.id === parseInt(formData.target_account_id, 10),
      );
      const carryingRate = getCurrencyCode(account) === 'IDR'
        ? 1
        : (account.balance > 0 && getBookBalanceInIDR(account) > 0
          ? getBookBalanceInIDR(account) / account.balance
          : (account.last_exchange_rate || 1));
      const bookImpactIDR = getCurrencyCode(account) === 'IDR'
        ? formData.amount
        : formData.amount * carryingRate;

      toast({
        title: 'Withdrawal Successful',
        description: (
          <Box>
            <Text fontSize="sm" fontWeight="bold">
              {formatNativeAmount(getCurrencyCode(account), formData.amount)} withdrawn from {account.name}
            </Text>
            <Text fontSize="xs" color={mutedTextColor} mt={1}>
              Debit: {selectedTargetAccount?.name || 'Counter account'} ({formatIDRAmount(bookImpactIDR)})
            </Text>
            <Text fontSize="xs" color={mutedTextColor}>
              Credit: {account.name} (-{formatNativeAmount(getCurrencyCode(account), formData.amount)})
            </Text>
            <Text fontSize="xs" color="orange.500" mt={1} fontWeight="bold">
              Double-entry balanced automatically.
            </Text>
          </Box>
        ),
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      handleClose();
    } catch (submitError: unknown) {
      console.error('Error processing withdrawal:', submitError);
      const normalizedError = submitError as ApiErrorShape;

      let errorMessage =
        normalizedError.response?.data?.details ||
        normalizedError.response?.data?.error ||
        normalizedError.response?.data?.message ||
        normalizedError.message ||
        'Failed to process withdrawal';
      let errorTitle = 'Transaction Failed';

      if (normalizedError.code === 'ECONNABORTED' || normalizedError.message?.includes('timeout') || normalizedError.message?.includes('exceeded')) {
        errorTitle = 'Transaction Timeout';
        errorMessage = 'The withdrawal operation timed out. Please wait a moment and check the account balance before retrying.';
      }

      setError(errorMessage);
      toast({
        title: errorTitle,
        description: errorMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setError(null);
    setFormData({
      date: new Date().toISOString().split('T')[0],
      amount: 0,
      reference: '',
      notes: '',
      purpose: 'NON_OPERATING_EXPENSE',
      target_account_id: '',
    });
    onClose();
  };

  if (!account) {
    return null;
  }

  const currencyCode = getCurrencyCode(account);
  const availableBalance = getAvailableBalance(account);
  const currentBookBalanceIDR = getBookBalanceInIDR(account);
  const currentBookRate = currencyCode === 'IDR'
    ? 1
    : (account.balance > 0 && currentBookBalanceIDR > 0 ? currentBookBalanceIDR / account.balance : (account.last_exchange_rate || 1));
  const bookImpactIDR = currencyCode === 'IDR' ? formData.amount : formData.amount * currentBookRate;
  const newBalance = account.balance - formData.amount;
  const newBookBalanceIDR = currentBookBalanceIDR - bookImpactIDR;
  const isForeignCurrencyAccount = isCashBankForeignCurrency(currencyCode);
  const selectedPurposeOption = withdrawalPurposeOptions.find(option => option.value === formData.purpose) || withdrawalPurposeOptions[0];
  const isOwnerDrawing = formData.purpose === 'OWNER_DRAWING';
  const normalizedRole = normalizeRole(user?.role);
  const canPostOwnerDrawing = isFinanceManagerRole(normalizedRole) || normalizedRole === 'director';
  const selectedTargetAccounts = getSelectedTargetAccounts();
  const selectedTargetAccount = selectedTargetAccounts.find(
    entry => entry.id === parseInt(formData.target_account_id || '0', 10),
  );
  const targetAccountLabel = formData.purpose === 'NON_OPERATING_EXPENSE'
    ? 'Debit Account (Non-Operating Expense)'
    : formData.purpose === 'OPERATING_EXPENSE'
      ? 'Debit Account (Operating Expense)'
      : formData.purpose === 'LIABILITY_PAYMENT'
        ? 'Debit Account (Liability)'
        : formData.purpose === 'OWNER_DRAWING'
          ? 'Debit Account (Pengambilan Pemilik / Prive)'
          : 'Debit Account (Asset)';
  const targetAccountPlaceholder = formData.purpose === 'NON_OPERATING_EXPENSE'
    ? 'Select 8xxx non-operating expense account'
    : formData.purpose === 'OPERATING_EXPENSE'
      ? 'Select operating expense account'
    : formData.purpose === 'LIABILITY_PAYMENT'
      ? 'Select liability account to reduce'
      : formData.purpose === 'OWNER_DRAWING'
        ? 'Pilih akun Prive / Owner Drawing'
        : 'Select asset account to increase';

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="2xl" scrollBehavior="inside">
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent bg={modalBg} border="1px solid" borderColor={borderColor}>
        <ModalHeader bg={modalHeaderBg} borderBottom="1px solid" borderColor={borderColor}>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">Withdrawal</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold" color={sectionTitleColor}>
                {t('cashBank.makeWithdrawal')}
              </Text>
              <Text fontSize="sm" color={helperTextColor} fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
            <Badge colorScheme="orange" variant="solid" fontSize="xs">
              AUTO DOUBLE ENTRY
            </Badge>
          </Flex>
        </ModalHeader>

        <ModalBody>
          {error && (
            <Alert status="error" mb={4} variant="subtle" borderRadius="md">
              <AlertIcon />
              <Box>
                <Text fontSize="sm" fontWeight="medium">
                  {error}
                </Text>
              </Box>
            </Alert>
          )}

          <VStack spacing={6} align="stretch">
            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                {t('cashBank.accountInformation')}
              </Text>
              <Box bg={panelBg} p={4} borderRadius="md" border="1px solid" borderColor={panelBorderColor}>
                <Flex justify="space-between" align="center" mb={2}>
                  <Box>
                    <Text fontSize="sm" color={mutedTextColor} mb={1}>Account Name</Text>
                    <Text fontWeight="medium" color={bodyTextColor}>{account.name}</Text>
                  </Box>
                  <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                    {account.type}
                  </Badge>
                </Flex>

                <Stat>
                  <StatLabel color={mutedTextColor}>Current Native Balance</StatLabel>
                  <StatNumber color={account.balance < 0 ? 'red.500' : 'green.600'} fontFamily="mono">
                    {formatNativeAmount(currencyCode, account.balance)}
                    {account.balance < 0 && ' (Dr)'}
                  </StatNumber>
                  <StatHelpText color={helperTextColor}>
                    {account.balance < 0 ? 'Overdraft' : account.balance > 0 ? 'Available' : 'Zero Balance'}
                  </StatHelpText>
                  <StatHelpText color={helperTextColor}>
                    Book balance for ledger: {formatIDRAmount(currentBookBalanceIDR)}
                  </StatHelpText>
                  <StatHelpText color="orange.500">
                    Available to use: {formatNativeAmount(currencyCode, availableBalance)}
                  </StatHelpText>
                </Stat>
              </Box>
            </Box>

            <Alert status={isForeignCurrencyAccount ? 'info' : 'success'} borderRadius="md" alignItems="flex-start">
              <AlertIcon mt={1} />
              <Box>
                <Flex gap={2} mb={2} wrap="wrap">
                  <Badge colorScheme="blue" variant="subtle">NATIVE {currencyCode}</Badge>
                  <Badge colorScheme="green" variant="subtle">BOOK IDR</Badge>
                </Flex>
                <Text fontSize="sm" fontWeight="medium">
                  {isForeignCurrencyAccount
                    ? `Withdrawal ${currencyCode} memengaruhi saldo native dalam ${currencyCode} dan book value ledger dalam IDR.`
                    : 'Untuk akun IDR, saldo rekening dan nilai buku ledger berkurang dengan angka yang sama.'}
                </Text>
                <Text fontSize="xs" color={mutedTextColor} mt={1}>
                  {isForeignCurrencyAccount
                    ? 'Sistem memakai carrying rate buku saat ini untuk melepas nilai buku IDR, jadi modal ini tidak meminta kurs baru.'
                    : 'Sistem hanya mengizinkan pemakaian saldo tersedia, bukan saldo yang sedang dibekukan.'}
                </Text>
              </Box>
            </Alert>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                {t('cashBank.transactionDetails')}
              </Text>

              <VStack spacing={4} align="stretch">
                <FormControl isRequired>
                  <FormLabel>{t('payments.form.paymentDate')}</FormLabel>
                  <Input
                    type="date"
                    value={formData.date}
                    onChange={(e) => handleInputChange('date', e.target.value)}
                    {...fieldStyles}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('payments.form.amount')} ({currencyCode})</FormLabel>
                  <NumberInput
                    value={formData.amount}
                    onChange={(_, value) => handleInputChange('amount', value || 0)}
                    min={0}
                    precision={getCashBankAmountPrecision(currencyCode)}
                  >
                    <NumberInputField fontFamily="mono" {...fieldStyles} />
                    <NumberInputStepper>
                      <NumberIncrementStepper />
                      <NumberDecrementStepper />
                    </NumberInputStepper>
                  </NumberInput>
                  {formData.amount > availableBalance && (
                    <Text fontSize="xs" color="red.500" mt={1}>
                      Amount exceeds available balance and cannot be posted.
                    </Text>
                  )}
                </FormControl>

                <FormControl isRequired={isOwnerDrawing}>
                  <FormLabel>{t('payments.form.referenceNumber')}</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={(e) => handleInputChange('reference', e.target.value)}
                    placeholder={t('payments.placeholders.referenceNumber')}
                    {...fieldStyles}
                  />
                  {isOwnerDrawing && (
                    <Text fontSize="xs" color={helperTextColor} mt={1}>
                      Nomor bukti atau referensi pengambilan wajib dicatat untuk audit trail.
                    </Text>
                  )}
                </FormControl>

                <FormControl isRequired={isOwnerDrawing}>
                  <FormLabel>{t('payments.form.notes')}</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={(e) => handleInputChange('notes', e.target.value)}
                    placeholder={t('payments.placeholders.notes')}
                    rows={3}
                    {...fieldStyles}
                  />
                  {isOwnerDrawing && (
                    <Text fontSize="xs" color={helperTextColor} mt={1}>
                      Jelaskan alasan dan pihak pemilik yang menerima pengambilan ini.
                    </Text>
                  )}
                </FormControl>
              </VStack>
            </Box>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                Posting Mapping
              </Text>
              <Text fontSize="sm" color={mutedTextColor} mb={3}>
                {isOwnerDrawing
                  ? 'Pilih akun Prive yang telah disetujui. Tidak semua akun Ekuitas dapat digunakan untuk pengambilan pemilik.'
                  : 'Pilih tujuan jurnal debit agar withdrawal langsung masuk ke operasional, non-operasional, liability, atau asset yang tepat.'}
              </Text>

              <VStack spacing={4} align="stretch">
                <FormControl isRequired>
                  <FormLabel>Withdrawal Purpose / Debit Group</FormLabel>
                  <Select
                    value={formData.purpose}
                    onChange={(e) => handlePurposeChange(e.target.value as WithdrawalPurpose)}
                    {...fieldStyles}
                  >
                    {withdrawalPurposeOptions.map(option => (
                      <option key={option.value} value={option.value}>
                        {option.label}
                      </option>
                    ))}
                  </Select>
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    {selectedPurposeOption.helper}
                  </Text>
                </FormControl>

                <Alert status="info" borderRadius="md" alignItems="flex-start">
                  <AlertIcon mt={1} />
                  <Box>
                    <AlertTitle fontSize="sm" mb={1}>
                      Mapping title
                    </AlertTitle>
                    <AlertDescription fontSize="sm">
                      <strong>{selectedPurposeOption.label}</strong> berarti <strong>{selectedPurposeOption.posting}</strong>. Ini membantu user melihat bahwa transaksi keluar tidak selalu berarti beban operasional.
                    </AlertDescription>
                  </Box>
                </Alert>

                {isOwnerDrawing && (
                  <Alert status="warning" borderRadius="md" alignItems="flex-start">
                    <AlertIcon mt={1} />
                    <Box>
                      <AlertTitle fontSize="sm" mb={1}>Kontrol Pengambilan Pemilik</AlertTitle>
                      <AlertDescription fontSize="sm">
                        Hanya Finance Manager atau Director yang dapat memposting transaksi ini. Referensi dan alasan wajib diisi; backend akan memvalidasi akun Prive yang dipilih.
                      </AlertDescription>
                      {!canPostOwnerDrawing && (
                        <Text fontSize="xs" mt={2} fontWeight="semibold">
                          Role Anda tidak dapat memposting Pengambilan Pemilik / Prive.
                        </Text>
                      )}
                    </Box>
                  </Alert>
                )}

                <FormControl isRequired>
                  <FormLabel>{targetAccountLabel}</FormLabel>
                  <Select
                    value={formData.target_account_id}
                    onChange={(e) => handleInputChange('target_account_id', e.target.value)}
                    placeholder={targetAccountPlaceholder}
                    isDisabled={loadingAccounts}
                    {...fieldStyles}
                  >
                    {selectedTargetAccounts.map(entry => (
                      <option key={`${formData.purpose}-${entry.id}`} value={entry.id}>
                        {entry.code} - {entry.name}
                        {typeof entry.balance === 'number' && entry.balance !== 0 && ` (Balance: ${entry.balance.toLocaleString('id-ID')})`}
                      </option>
                    ))}
                  </Select>
                  <Text fontSize="xs" color={helperTextColor} mt={1}>
                    {isOwnerDrawing
                      ? 'Daftar ini hanya berisi akun Prive/Owner Drawing aktif yang diizinkan oleh server.'
                      : 'Akun lawan jurnal wajib dipilih supaya sistem tidak jatuh ke fallback expense yang generik.'}
                  </Text>
                  {loadingAccounts && (
                    <Text fontSize="xs" color={helperTextColor} mt={1}>
                      Loading available target accounts...
                    </Text>
                  )}
                </FormControl>

                {!loadingAccounts && selectedTargetAccounts.length === 0 && (
                  <Alert status="warning" borderRadius="md">
                    <AlertIcon />
                    <AlertDescription fontSize="sm">
                      {isOwnerDrawing
                        ? 'Belum ada akun Prive/Owner Drawing aktif yang disetujui. Hubungi Administrator/Accounting untuk menandai akun yang tepat; jangan gunakan akun Ekuitas lain sebagai pengganti.'
                        : `Belum ada akun aktif yang cocok untuk ${selectedPurposeOption.label.toLowerCase()}. Tambahkan akun COA terlebih dahulu sebelum memproses withdrawal ini.`}
                    </AlertDescription>
                  </Alert>
                )}
              </VStack>
            </Box>

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                  Double-Entry Preview
                </Text>
                <Box bg={previewBg} p={4} borderRadius="md" border="1px solid" borderColor={previewBorderColor}>
                  <VStack spacing={3} align="stretch">
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color="orange.600">
                        Journal Entry (Auto-Generated)
                      </Text>
                      <Badge colorScheme="orange" variant="solid" fontSize="xs">
                        BALANCED
                      </Badge>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="green" variant="outline" fontSize="xs">DR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>
                            {selectedTargetAccount?.name || selectedPurposeOption.label}
                          </Text>
                          <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">
                            {selectedTargetAccount?.code || '-'}
                          </Text>
                          <Text fontSize="xs" color={helperTextColor}>
                            {selectedPurposeOption.posting}
                          </Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="green.600">
                        +{formatIDRAmount(bookImpactIDR)}
                      </Text>
                    </HStack>

                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="orange" variant="outline" fontSize="xs">CR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium" color={bodyTextColor}>{account.name}</Text>
                          <Text fontSize="xs" color={mutedTextColor} fontFamily="mono">{account.code}</Text>
                          <Text fontSize="xs" color={helperTextColor}>Saldo rekening berkurang</Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="orange.600">
                        -{formatNativeAmount(currencyCode, formData.amount)}
                      </Text>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color={bodyTextColor}>Total Balance</Text>
                      <Text fontSize="sm" fontWeight="bold" color="green.600">
                        DR {formatIDRAmount(bookImpactIDR)} = CR {formatIDRAmount(bookImpactIDR)}
                      </Text>
                    </HStack>
                  </VStack>
                </Box>
              </Box>
            )}

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color={sectionTitleColor} mb={3}>
                  Balance Preview
                </Text>
                <Alert status={newBalance < 0 ? 'warning' : 'info'} borderRadius="md">
                  <AlertIcon />
                  <Box w="full">
                    <SimpleGrid columns={{ base: 1, md: 3 }} spacing={3}>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>Native amount out</Text>
                        <Text fontSize="sm" fontWeight="bold" color="orange.600" fontFamily="mono">
                          -{formatNativeAmount(currencyCode, formData.amount)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>Book amount released</Text>
                        <Text fontSize="sm" fontWeight="bold" color="blue.600" fontFamily="mono">
                          {formatIDRAmount(bookImpactIDR)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color={mutedTextColor}>Ending native balance</Text>
                        <Text fontSize="sm" fontWeight="bold" color={newBalance < 0 ? 'red.500' : 'green.600'} fontFamily="mono">
                          {formatNativeAmount(currencyCode, newBalance)}
                        </Text>
                      </Box>
                    </SimpleGrid>
                    <Text fontSize="sm" mt={3} color={bodyTextColor}>
                      Ending book balance: {formatIDRAmount(newBookBalanceIDR)}
                    </Text>
                    {isForeignCurrencyAccount && (
                      <Text fontSize="xs" color={mutedTextColor} mt={1}>
                        Carrying rate used for ledger release: 1 {currencyCode} = {currentBookRate.toLocaleString('id-ID', {
                          minimumFractionDigits: 2,
                          maximumFractionDigits: 4,
                        })} IDR
                      </Text>
                    )}
                  </Box>
                </Alert>
              </Box>
            )}
          </VStack>
        </ModalBody>

        <ModalFooter bg={modalFooterBg} borderTop="1px solid" borderColor={borderColor}>
          <Button variant="ghost" mr={3} onClick={handleClose} isDisabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button
            colorScheme="orange"
            onClick={handleSubmit}
            isLoading={loading}
            loadingText={t('common.processing')}
            isDisabled={
              formData.amount <= 0 ||
              formData.amount > availableBalance ||
              !formData.target_account_id ||
              (isOwnerDrawing && (!canPostOwnerDrawing || !formData.reference.trim() || !formData.notes.trim()))
            }
          >
            {t('cashBank.processWithdrawal')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default DepositWithdrawalForm;
