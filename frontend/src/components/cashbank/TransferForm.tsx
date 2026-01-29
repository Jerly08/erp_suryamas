'use client';

import React, { useState, useEffect } from 'react';
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
  Select,
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
  SimpleGrid,
  Card,
  CardBody,
} from '@chakra-ui/react';
import { CashBank, TransferRequest } from '@/services/cashbankService';
import cashbankService from '@/services/cashbankService';
import { useTranslation } from '@/hooks/useTranslation';

interface TransferFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  sourceAccount: CashBank | null;
}

const TransferForm: React.FC<TransferFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  sourceAccount
}) => {
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    to_account_id: 0,
    exchange_rate: 1,
    reference: '',
    notes: ''
  });

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [accounts, setAccounts] = useState<CashBank[]>([]);

  const [selectedDestAccount, setSelectedDestAccount] = useState<CashBank | null>(null);
  const toast = useToast();
  const { t } = useTranslation();

  // Load available accounts for transfer destination
  useEffect(() => {
    const loadAccounts = async () => {
      if (isOpen && sourceAccount) {
        try {
          const allAccounts = await cashbankService.getCashBankAccounts();
          // Filter out the source account and inactive accounts
          const availableAccounts = allAccounts.filter(acc =>
            acc.id !== sourceAccount.id && acc.is_active
          );
          setAccounts(availableAccounts);
        } catch (error) {
          console.error('Error loading accounts:', error);
        }
      }
    };

    loadAccounts();
  }, [isOpen, sourceAccount]);

  const handleInputChange = (field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value
    }));
    setError(null);

    // Update selected destination account when to_account_id changes
    if (field === 'to_account_id') {
      const destAccount = accounts.find(acc => acc.id === value);
      setSelectedDestAccount(destAccount || null);

      // Auto-set exchange rate if currencies are different
      if (destAccount && sourceAccount) {
        if (sourceAccount.currency !== destAccount.currency) {
          // In a real app, you'd fetch real exchange rates
          setFormData(prev => ({ ...prev, exchange_rate: 1 }));
        } else {
          setFormData(prev => ({ ...prev, exchange_rate: 1 }));
        }
      }
    }
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      // Basic validation
      if (!sourceAccount) {
        throw new Error('No source account selected');
      }

      if (!selectedDestAccount) {
        throw new Error('Please select destination account');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      if (formData.amount > sourceAccount.balance) {
        throw new Error(`Insufficient balance. Available: ${sourceAccount.currency} ${sourceAccount.balance.toLocaleString('id-ID')}`);
      }

      if (sourceAccount.currency !== selectedDestAccount.currency && formData.exchange_rate <= 0) {
        throw new Error('Exchange rate is required for different currencies');
      }

      // Prepare request data
      const requestData: TransferRequest = {
        from_account_id: sourceAccount.id,
        to_account_id: formData.to_account_id,
        date: formData.date,
        amount: formData.amount,
        exchange_rate: formData.exchange_rate,
        reference: formData.reference,
        notes: formData.notes
      };

      await cashbankService.processTransfer(requestData);

      toast({
        title: 'Transfer Successful',
        description: `${sourceAccount.currency} ${formData.amount.toLocaleString('id-ID')} transferred from ${sourceAccount.name} to ${selectedDestAccount.name}`,
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      onClose();
    } catch (err: any) {
      console.error('Error processing transfer:', err);
      setError(err.response?.data?.details || err.message || 'Failed to process transfer');
      toast({
        title: 'Transfer Failed',
        description: err.response?.data?.details || err.message || 'Failed to process transfer',
        status: 'error',
        duration: 5000,
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
      to_account_id: 0,
      exchange_rate: 1,
      reference: '',
      notes: ''
    });
    setSelectedDestAccount(null);
    onClose();
  };

  if (!sourceAccount) return null;

  const convertedAmount = formData.amount * formData.exchange_rate;
  const requiresExchangeRate = selectedDestAccount &&
    sourceAccount.currency !== selectedDestAccount.currency;

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="xl">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">🔄</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold">
                {t('cashBank.transferFunds')}
              </Text>
              <Text fontSize="sm" color="gray.500">
                {t('cashBank.moveMoneyDesc')}
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
            {/* Source and Destination Account Information */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                🏦 {t('cashBank.accountSelection')}
              </Text>

              <SimpleGrid columns={2} spacing={4}>
                {/* Source Account */}
                <Card>
                  <CardBody>
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('cashBank.fromSource')}</Text>
                    <Flex alignItems="center" gap={2} mb={2}>
                      <Badge colorScheme={sourceAccount.type === 'CASH' ? 'green' : 'blue'} size="sm">
                        {sourceAccount.type}
                      </Badge>
                      <Text fontWeight="medium" fontSize="sm">{sourceAccount.name}</Text>
                    </Flex>

                    <Stat size="sm">
                      <StatLabel>{t('cashBank.availableBalance')}</StatLabel>
                      <StatNumber
                        color={sourceAccount.balance < 0 ? 'red.500' : 'green.600'}
                        fontFamily="mono"
                        fontSize="md"
                      >
                        {sourceAccount.currency} {Math.abs(sourceAccount.balance).toLocaleString('id-ID')}
                        {sourceAccount.balance < 0 && ' (Dr)'}
                      </StatNumber>
                    </Stat>
                  </CardBody>
                </Card>

                {/* Destination Account */}
                <Card>
                  <CardBody>
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('cashBank.toDestination')}</Text>
                    <FormControl isRequired>
                      <Select
                        value={formData.to_account_id}
                        onChange={(e) => handleInputChange('to_account_id', parseInt(e.target.value))}
                        placeholder={t('cashBank.selectGLAccountPlaceholder')}
                        size="sm"
                      >
                        {accounts.map((acc) => (
                          <option key={acc.id} value={acc.id}>
                            [{acc.type}] {acc.name} - {acc.currency} {acc.balance.toLocaleString('id-ID')}
                          </option>
                        ))}
                      </Select>
                    </FormControl>

                    {selectedDestAccount && (
                      <Box mt={2}>
                        <Flex alignItems="center" gap={2} mb={1}>
                          <Badge colorScheme={selectedDestAccount.type === 'CASH' ? 'green' : 'blue'} size="sm">
                            {selectedDestAccount.type}
                          </Badge>
                          <Text fontSize="xs" color="gray.600">{selectedDestAccount.name}</Text>
                        </Flex>
                        <Text fontSize="xs" color="gray.500" fontFamily="mono">
                          Balance: {selectedDestAccount.currency} {selectedDestAccount.balance.toLocaleString('id-ID')}
                        </Text>
                      </Box>
                    )}
                  </CardBody>
                </Card>
              </SimpleGrid>
            </Box>

            {/* Transfer Details */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                📝 {t('cashBank.transferDetails')}
              </Text>

              <VStack spacing={4} align="stretch">
                <SimpleGrid columns={2} spacing={4}>
                  <FormControl isRequired>
                    <FormLabel>{t('cashBank.transferDate')}</FormLabel>
                    <Input
                      type="date"
                      value={formData.date}
                      onChange={(e) => handleInputChange('date', e.target.value)}
                    />
                  </FormControl>

                  <FormControl isRequired>
                    <FormLabel>{t('payments.form.amount')} ({sourceAccount.currency})</FormLabel>
                    <NumberInput
                      value={formData.amount}
                      onChange={(_, value) => handleInputChange('amount', value || 0)}
                      min={0}
                      precision={2}
                      max={sourceAccount.balance}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                    {formData.amount > sourceAccount.balance && (
                      <Text fontSize="xs" color="red.500" mt={1}>
                        ⚠️ Amount exceeds available balance
                      </Text>
                    )}
                  </FormControl>
                </SimpleGrid>

                {/* Exchange Rate (only if different currencies) */}
                {requiresExchangeRate && (
                  <Alert status="info" borderRadius="md">
                    <AlertIcon />
                    <Box>
                      <Text fontSize="sm" fontWeight="medium" mb={2}>
                        Currency Conversion Required
                      </Text>
                      <Text fontSize="xs" color="gray.600" mb={2}>
                        {sourceAccount.currency} → {selectedDestAccount?.currency}
                      </Text>
                      <FormControl>
                        <FormLabel fontSize="sm">{t('cashBank.exchangeRate')}</FormLabel>
                        <NumberInput
                          value={formData.exchange_rate}
                          onChange={(_, value) => handleInputChange('exchange_rate', value || 1)}
                          min={0}
                          precision={6}
                          step={0.01}
                        >
                          <NumberInputField />
                          <NumberInputStepper>
                            <NumberIncrementStepper />
                            <NumberDecrementStepper />
                          </NumberInputStepper>
                        </NumberInput>
                        <Text fontSize="xs" color="gray.500" mt={1}>
                          1 {sourceAccount.currency} = {formData.exchange_rate} {selectedDestAccount?.currency}
                        </Text>
                      </FormControl>
                    </Box>
                  </Alert>
                )}

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

            {/* Transfer Preview */}
            {formData.amount > 0 && selectedDestAccount && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  💡 Transfer Preview
                </Text>
                <Alert status="info" borderRadius="md">
                  <AlertIcon />
                  <VStack align="start" spacing={2}>
                    <Text fontSize="sm" fontWeight="medium">
                      Transfer Summary:
                    </Text>
                    <SimpleGrid columns={2} spacing={4} w="full">
                      <Box>
                        <Text fontSize="xs" color="gray.600">From: {sourceAccount.name}</Text>
                        <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="red.600">
                          -{sourceAccount.currency} {formData.amount.toLocaleString('id-ID')}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          New balance: {sourceAccount.currency} {(sourceAccount.balance - formData.amount).toLocaleString('id-ID')}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color="gray.600">To: {selectedDestAccount.name}</Text>
                        <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="green.600">
                          +{selectedDestAccount.currency} {convertedAmount.toLocaleString('id-ID')}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          New balance: {selectedDestAccount.currency} {(selectedDestAccount.balance + convertedAmount).toLocaleString('id-ID')}
                        </Text>
                      </Box>
                    </SimpleGrid>
                    {requiresExchangeRate && (
                      <Text fontSize="xs" color="orange.600">
                        💱 Exchange rate applied: {formData.exchange_rate}
                      </Text>
                    )}
                  </VStack>
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
            colorScheme="blue"
            onClick={handleSubmit}
            isLoading={loading}
            loadingText={t('common.processing')}
            isDisabled={
              formData.amount <= 0 ||
              !selectedDestAccount ||
              formData.amount > sourceAccount.balance ||
              (requiresExchangeRate && formData.exchange_rate <= 0)
            }
          >
            {t('cashBank.transferFunds')}
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default TransferForm;
