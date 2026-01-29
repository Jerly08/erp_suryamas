'use client';

import React, { useState } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Button,
  FormControl,
  FormLabel,
  Input,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Alert,
  AlertIcon,
  useToast,
  Box,
  Text,
  VStack,
  Flex,
  Badge,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
} from '@chakra-ui/react';
import { CashBank, DepositRequest, WithdrawalRequest, ManualJournalEntry } from '@/services/cashbankService';
import cashbankService from '@/services/cashbankService';
import JournalEntryForm from './JournalEntryForm';
import { useTranslation } from '@/hooks/useTranslation';

interface DepositWithdrawalFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account: CashBank | null;
  mode: 'deposit' | 'withdrawal';
}

const DepositWithdrawalForm: React.FC<DepositWithdrawalFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  account,
  mode
}) => {
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    reference: '',
    notes: ''
  });

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [journalEntries, setJournalEntries] = useState<ManualJournalEntry[]>([]);
  const [manualJournalMode, setManualJournalMode] = useState(false);

  const [journalErrors, setJournalErrors] = useState<{ [key: number]: string }>({});
  const toast = useToast();
  const { t } = useTranslation();

  const handleInputChange = (field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value
    }));
    setError(null);
  };

  const validateJournalEntries = (): boolean => {
    if (!manualJournalMode) return true;

    const errors: { [key: number]: string } = {};
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
        errors[index] = entryErrors.join(', ');
        hasErrors = true;
      }
    });

    // Check if debits equal credits
    const totalDebit = journalEntries.reduce((sum, entry) => sum + entry.debit_amount, 0);
    const totalCredit = journalEntries.reduce((sum, entry) => sum + entry.credit_amount, 0);

    if (Math.abs(totalDebit - totalCredit) > 0.01) {
      setError('Journal entries are not balanced. Total debits must equal total credits.');
      hasErrors = true;
    }

    setJournalErrors(errors);
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

      // Show progress toast for long operations
      toast({
        title: 'Processing Transaction',
        description: 'Please wait while we process your transaction. This may take up to 60 seconds.',
        status: 'info',
        duration: 5000,
        isClosable: true,
      });

      // Basic validation
      if (!account) {
        throw new Error('No account selected');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      if (mode === 'withdrawal' && formData.amount > account.balance) {
        throw new Error(`Insufficient balance. Available: ${account.currency} ${account.balance.toLocaleString('id-ID')}`);
      }

      // Validate journal entries if manual mode is enabled
      if (manualJournalMode) {
        if (!validateJournalEntries()) {
          return; // Validation failed, errors are already set
        }

        if (journalEntries.length === 0) {
          setError('At least one journal entry is required in manual mode');
          return;
        }
      }

      // Prepare request data
      const requestData: DepositRequest | WithdrawalRequest = {
        account_id: account.id,
        date: formData.date,
        amount: formData.amount,
        reference: formData.reference,
        notes: formData.notes,
        ...(manualJournalMode && journalEntries.length > 0 ? { journal_entries: journalEntries } : {})
      };

      if (mode === 'deposit') {
        await cashbankService.processDeposit(requestData as DepositRequest);
        toast({
          title: 'Deposit Successful',
          description: `${account.currency} ${formData.amount.toLocaleString('id-ID')} has been deposited to ${account.name}${manualJournalMode ? ' with manual journal entries' : ''
            }`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else {
        await cashbankService.processWithdrawal(requestData as WithdrawalRequest);
        toast({
          title: 'Withdrawal Successful',
          description: `${account.currency} ${formData.amount.toLocaleString('id-ID')} has been withdrawn from ${account.name}${manualJournalMode ? ' with manual journal entries' : ''
            }`,
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }

      onSuccess();
      onClose();
    } catch (err: any) {
      console.error(`Error processing ${mode}:`, err);

      let errorMessage = err.response?.data?.details || err.message || `Failed to process ${mode}`;
      let errorTitle = 'Transaction Failed';

      // Handle timeout errors specifically
      if (err.code === 'ECONNABORTED' || err.message?.includes('timeout') || err.message?.includes('exceeded')) {
        errorTitle = 'Transaction Timeout';
        errorMessage = `The ${mode} operation timed out. This might happen due to high server load. Please wait a moment and check your account balance before retrying.`;
      }

      setError(errorMessage);
      toast({
        title: errorTitle,
        description: errorMessage,
        status: 'error',
        duration: 8000, // Longer duration for timeout errors
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
      notes: ''
    });
    onClose();
  };

  if (!account) return null;

  const newBalance = mode === 'deposit'
    ? account.balance + formData.amount
    : account.balance - formData.amount;

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size={manualJournalMode ? "6xl" : "lg"} scrollBehavior="inside">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">{mode === 'deposit' ? '💰' : '💸'}</Text>
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
            {/* Account Information */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                💼 {t('cashBank.accountInformation')}
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
                  <StatLabel>{t('cashBank.currentBalance')}</StatLabel>
                  <StatNumber
                    color={account.balance < 0 ? 'red.500' : 'green.600'}
                    fontFamily="mono"
                  >
                    {account.currency} {Math.abs(account.balance).toLocaleString('id-ID')}
                    {account.balance < 0 && ' (Dr)'}
                  </StatNumber>
                  <StatHelpText>
                    {account.balance < 0 ? '⚠️ Overdraft' :
                      account.balance > 0 ? '✅ Available' : '➖ Zero Balance'}
                  </StatHelpText>
                </Stat>
              </Box>
            </Box>

            {/* Transaction Form */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                📝 {t('cashBank.transactionDetails')}
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
                  <FormLabel>{t('payments.form.amount')} ({account.currency})</FormLabel>
                  <NumberInput
                    value={formData.amount}
                    onChange={(_, value) => handleInputChange('amount', value || 0)}
                    min={0}
                    precision={2}
                  >
                    <NumberInputField />
                    <NumberInputStepper>
                      <NumberIncrementStepper />
                      <NumberDecrementStepper />
                    </NumberInputStepper>
                  </NumberInput>
                  {mode === 'withdrawal' && formData.amount > account.balance && (
                    <Text fontSize="xs" color="red.500" mt={1}>
                      ⚠️ Amount exceeds available balance
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

            {/* Journal Entry Form */}
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

            {/* Balance Preview */}
            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  💡 Balance Preview
                </Text>
                <Alert status={newBalance < 0 ? 'warning' : 'success'} borderRadius="md">
                  <AlertIcon />
                  <Box>
                    <Text fontSize="sm" fontWeight="medium">
                      New balance after {mode}:
                    </Text>
                    <Text fontSize="lg" fontWeight="bold" fontFamily="mono" mt={1}>
                      {account.currency} {Math.abs(newBalance).toLocaleString('id-ID')}
                      {newBalance < 0 && ' (Dr)'}
                    </Text>
                    {newBalance < 0 && (
                      <Text fontSize="xs" color="orange.600" mt={1}>
                        ⚠️ This will result in an overdraft
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
            isDisabled={formData.amount <= 0 || (mode === 'withdrawal' && formData.amount > account.balance)}
          >
            {mode === 'deposit' ? t('cashBank.processDeposit') : t('cashBank.processWithdrawal')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default DepositWithdrawalForm;
