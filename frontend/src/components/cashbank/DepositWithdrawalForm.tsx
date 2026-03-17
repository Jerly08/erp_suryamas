'use client';

import React, { useState } from 'react';
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  Button,
  Flex,
  FormControl,
  FormLabel,
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
  Stat,
  StatHelpText,
  StatLabel,
  StatNumber,
  Text,
  Textarea,
  useToast,
  VStack,
} from '@chakra-ui/react';
import cashbankService, {
  CashBank,
  DepositRequest,
  getAvailableBalance,
  getBookBalanceInIDR,
  ManualJournalEntry,
  WithdrawalRequest,
} from '@/services/cashbankService';
import JournalEntryForm from './JournalEntryForm';
import { useTranslation } from '@/hooks/useTranslation';

interface DepositWithdrawalFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account: CashBank | null;
  mode: 'deposit' | 'withdrawal';
}

interface ApiErrorShape {
  response?: {
    data?: {
      details?: string;
      message?: string;
    };
  };
  code?: string;
  message?: string;
}

const getCurrencyCode = (account?: CashBank | null): string => (account?.currency || 'IDR').toUpperCase();

const formatNativeAmount = (currency: string, amount: number): string => `${currency} ${Math.abs(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: currency === 'USD' ? 2 : 0,
  maximumFractionDigits: currency === 'USD' ? 2 : 0,
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
  mode,
}) => {
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    reference: '',
    notes: '',
  });
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [journalEntries, setJournalEntries] = useState<ManualJournalEntry[]>([]);
  const [manualJournalMode, setManualJournalMode] = useState(false);
  const [journalErrors, setJournalErrors] = useState<{ [key: number]: string }>({});

  const toast = useToast();
  const { t } = useTranslation();

  const handleInputChange = (field: string, value: string | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
    setError(null);
  };

  const validateJournalEntries = (): boolean => {
    if (!manualJournalMode) {
      return true;
    }

    const nextErrors: { [key: number]: string } = {};
    let hasErrors = false;

    journalEntries.forEach((entry, index) => {
      const entryErrors: string[] = [];

      if (!entry.account_id || entry.account_id === 0) {
        entryErrors.push('Account is required');
      }
      if (!entry.description || entry.description.trim() === '') {
        entryErrors.push('Description is required');
      }
      if (entry.debit_amount === 0 && entry.credit_amount === 0) {
        entryErrors.push('Either debit or credit amount must be greater than zero');
      }
      if (entry.debit_amount > 0 && entry.credit_amount > 0) {
        entryErrors.push('Entry cannot have both debit and credit amounts');
      }

      if (entryErrors.length > 0) {
        nextErrors[index] = entryErrors.join(', ');
        hasErrors = true;
      }
    });

    const totalDebit = journalEntries.reduce((sum, entry) => sum + entry.debit_amount, 0);
    const totalCredit = journalEntries.reduce((sum, entry) => sum + entry.credit_amount, 0);
    if (Math.abs(totalDebit - totalCredit) > 0.01) {
      setError('Journal entries are not balanced. Total debits must equal total credits.');
      hasErrors = true;
    }

    setJournalErrors(nextErrors);
    return !hasErrors;
  };

  const handleJournalEntriesChange = (entries: ManualJournalEntry[]) => {
    setJournalEntries(entries);
    setJournalErrors({});
    setError(null);
  };

  const handleJournalModeToggle = (enabled: boolean) => {
    setManualJournalMode(enabled);
    if (!enabled) {
      setJournalEntries([]);
      setJournalErrors({});
    }
    setError(null);
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      toast({
        title: 'Processing Transaction',
        description: 'Please wait while we process your transaction. This may take up to 60 seconds.',
        status: 'info',
        duration: 5000,
        isClosable: true,
      });

      if (!account) {
        throw new Error('No account selected');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      if (mode === 'withdrawal' && formData.amount > getAvailableBalance(account)) {
        throw new Error(`Insufficient balance. Available: ${formatNativeAmount(getCurrencyCode(account), getAvailableBalance(account))}`);
      }

      if (manualJournalMode) {
        if (!validateJournalEntries()) {
          return;
        }

        if (journalEntries.length === 0) {
          setError('At least one journal entry is required in manual mode');
          return;
        }
      }

      const requestData: DepositRequest | WithdrawalRequest = {
        account_id: account.id,
        date: formData.date,
        amount: formData.amount,
        reference: formData.reference,
        notes: formData.notes,
        ...(manualJournalMode && journalEntries.length > 0 ? { journal_entries: journalEntries } : {}),
      };

      if (mode === 'deposit') {
        await cashbankService.processDeposit(requestData as DepositRequest);
        toast({
          title: 'Deposit Successful',
          description: `${formatNativeAmount(getCurrencyCode(account), formData.amount)} has been deposited to ${account.name}${manualJournalMode ? ' with manual journal entries' : ''}`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else {
        await cashbankService.processWithdrawal(requestData as WithdrawalRequest);
        toast({
          title: 'Withdrawal Successful',
          description: `${formatNativeAmount(getCurrencyCode(account), formData.amount)} has been withdrawn from ${account.name}${manualJournalMode ? ' with manual journal entries' : ''}`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }

      onSuccess();
      onClose();
    } catch (submitError: unknown) {
      console.error(`Error processing ${mode}:`, submitError);
      const normalizedError = submitError as ApiErrorShape;

      let errorMessage = normalizedError.response?.data?.details || normalizedError.message || `Failed to process ${mode}`;
      let errorTitle = 'Transaction Failed';

      if (normalizedError.code === 'ECONNABORTED' || normalizedError.message?.includes('timeout') || normalizedError.message?.includes('exceeded')) {
        errorTitle = 'Transaction Timeout';
        errorMessage = `The ${mode} operation timed out. Please wait a moment and check the account balance before retrying.`;
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
    setJournalErrors({});
    setJournalEntries([]);
    setManualJournalMode(false);
    setFormData({
      date: new Date().toISOString().split('T')[0],
      amount: 0,
      reference: '',
      notes: '',
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
  const newBalance = mode === 'deposit'
    ? account.balance + formData.amount
    : account.balance - formData.amount;
  const newBookBalanceIDR = mode === 'deposit'
    ? currentBookBalanceIDR + bookImpactIDR
    : currentBookBalanceIDR - bookImpactIDR;
  const isUsdAccount = currencyCode === 'USD';

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size={manualJournalMode ? '6xl' : 'lg'} scrollBehavior="inside">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">{mode === 'deposit' ? 'Deposit' : 'Withdrawal'}</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold">
                {mode === 'deposit' ? t('cashBank.makeDeposit') : t('cashBank.makeWithdrawal')}
              </Text>
              <Text fontSize="sm" color="gray.500" fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
          </Flex>
        </ModalHeader>

        <ModalBody>
          {error && (
            <Alert status="error" mb={4}>
              <AlertIcon />
              {error}
            </Alert>
          )}

          <VStack spacing={6} align="stretch">
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                {t('cashBank.accountInformation')}
              </Text>
              <Box bg="gray.50" p={4} borderRadius="md">
                <Flex justify="space-between" align="center" mb={2}>
                  <Box>
                    <Text fontSize="sm" color="gray.600" mb={1}>Account Name</Text>
                    <Text fontWeight="medium">{account.name}</Text>
                  </Box>
                  <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                    {account.type}
                  </Badge>
                </Flex>

                <Stat>
                  <StatLabel>Current Native Balance</StatLabel>
                  <StatNumber color={account.balance < 0 ? 'red.500' : 'green.600'} fontFamily="mono">
                    {formatNativeAmount(currencyCode, account.balance)}
                    {account.balance < 0 && ' (Dr)'}
                  </StatNumber>
                  <StatHelpText>
                    {account.balance < 0 ? 'Overdraft' : account.balance > 0 ? 'Available' : 'Zero Balance'}
                  </StatHelpText>
                  <StatHelpText color="gray.500">
                    Book balance for ledger: {formatIDRAmount(currentBookBalanceIDR)}
                  </StatHelpText>
                  {mode === 'withdrawal' && (
                    <StatHelpText color="orange.500">
                      Available to use: {formatNativeAmount(currencyCode, availableBalance)}
                    </StatHelpText>
                  )}
                </Stat>
              </Box>
            </Box>

            <Alert status={isUsdAccount ? 'info' : 'success'} borderRadius="md" alignItems="flex-start">
              <AlertIcon mt={1} />
              <Box>
                <Flex gap={2} mb={2} wrap="wrap">
                  <Badge colorScheme="blue" variant="subtle">NATIVE {currencyCode}</Badge>
                  <Badge colorScheme="green" variant="subtle">BOOK IDR</Badge>
                </Flex>
                <Text fontSize="sm" fontWeight="medium">
                  {isUsdAccount
                    ? 'Transaksi USD akan memengaruhi saldo rekening dalam USD dan nilai buku ledger dalam IDR.'
                    : 'Untuk akun IDR, saldo rekening dan nilai buku ledger berubah dengan angka yang sama.'}
                </Text>
                <Text fontSize="xs" color="gray.600" mt={1}>
                  {mode === 'withdrawal' && isUsdAccount
                    ? 'Withdrawal USD tidak meminta kurs baru. Sistem memakai carrying rate buku saat ini untuk melepas nilai buku IDR.'
                    : mode === 'withdrawal'
                      ? 'Withdrawal hanya bisa memakai saldo tersedia, bukan total saldo yang sedang terblokir.'
                      : 'Deposit pada form ini mengikuti nilai buku berdasarkan carrying rate akun saat ini.'}
                </Text>
              </Box>
            </Alert>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                {t('cashBank.transactionDetails')}
              </Text>

              <VStack spacing={4} align="stretch">
                <FormControl isRequired>
                  <FormLabel>{t('payments.form.paymentDate')}</FormLabel>
                  <Input
                    type="date"
                    value={formData.date}
                    onChange={(e) => handleInputChange('date', e.target.value)}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>{t('payments.form.amount')} ({currencyCode})</FormLabel>
                  <NumberInput
                    value={formData.amount}
                    onChange={(_, value) => handleInputChange('amount', value || 0)}
                    min={0}
                    precision={currencyCode === 'USD' ? 2 : 0}
                  >
                    <NumberInputField fontFamily="mono" />
                    <NumberInputStepper>
                      <NumberIncrementStepper />
                      <NumberDecrementStepper />
                    </NumberInputStepper>
                  </NumberInput>
                  {mode === 'withdrawal' && formData.amount > availableBalance && (
                    <Text fontSize="xs" color="red.500" mt={1}>
                      Amount exceeds available balance and cannot be posted
                    </Text>
                  )}
                </FormControl>

                <FormControl>
                  <FormLabel>{t('payments.form.referenceNumber')}</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={(e) => handleInputChange('reference', e.target.value)}
                    placeholder={t('payments.placeholders.referenceNumber')}
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>{t('payments.form.notes')}</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={(e) => handleInputChange('notes', e.target.value)}
                    placeholder={t('payments.placeholders.notes')}
                    rows={3}
                  />
                </FormControl>
              </VStack>
            </Box>

            <JournalEntryForm
              journalEntries={journalEntries}
              onChange={handleJournalEntriesChange}
              isEnabled={manualJournalMode}
              onToggle={handleJournalModeToggle}
              transactionAmount={formData.amount}
              mode={mode}
              cashBankAccountId={account.account_id}
              errors={journalErrors}
            />

            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  Balance Preview
                </Text>
                <Alert status={mode === 'withdrawal' && newBalance < 0 ? 'warning' : 'info'} borderRadius="md">
                  <AlertIcon />
                  <Box>
                    <Text fontSize="sm" fontWeight="medium">
                      Ending native balance: {formatNativeAmount(currencyCode, newBalance)}
                    </Text>
                    <Text fontSize="sm" mt={1}>
                      Book amount impact: {formatIDRAmount(bookImpactIDR)}
                    </Text>
                    <Text fontSize="sm" mt={1}>
                      Ending book balance: {formatIDRAmount(newBookBalanceIDR)}
                    </Text>
                    {currencyCode === 'USD' && (
                      <Text fontSize="xs" color="gray.600" mt={1}>
                        Carrying rate used for ledger release: 1 USD = {currentBookRate.toLocaleString('id-ID', {
                          minimumFractionDigits: 2,
                          maximumFractionDigits: 4,
                        })} IDR
                      </Text>
                    )}
                    {mode === 'withdrawal' && (
                      <Text fontSize="xs" color="gray.600" mt={1}>
                        Sistem akan menolak transaksi jika nominal melebihi saldo tersedia.
                      </Text>
                    )}
                  </Box>
                </Alert>
              </Box>
            )}
          </VStack>
        </ModalBody>

        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={handleClose} isDisabled={loading}>
            {t('common.cancel')}
          </Button>
          <Button
            colorScheme={mode === 'deposit' ? 'green' : 'orange'}
            onClick={handleSubmit}
            isLoading={loading}
            loadingText={t('common.processing')}
            isDisabled={formData.amount <= 0 || (mode === 'withdrawal' && formData.amount > availableBalance)}
          >
            {mode === 'deposit' ? t('cashBank.processDeposit') : t('cashBank.processWithdrawal')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default DepositWithdrawalForm;
