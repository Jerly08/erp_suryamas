'use client';

import React, { useEffect, useState } from 'react';
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
import {
  CashBank,
  TransferRequest,
  formatCashBankAmount,
  getAvailableBalance,
  getBookBalanceInIDR,
} from '@/services/cashbankService';
import cashbankService from '@/services/cashbankService';
import { useTranslation } from '@/hooks/useTranslation';

interface TransferFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  sourceAccount: CashBank | null;
}

const getCurrencyCode = (account?: CashBank | null): string => (account?.currency || 'IDR').toUpperCase();

const getEffectiveBookRate = (account: CashBank): number => {
  if (getCurrencyCode(account) === 'IDR') {
    return 1;
  }

  const nativeBalance = account.balance || 0;
  const bookBalance = getBookBalanceInIDR(account);
  const derivedRate = nativeBalance !== 0 ? bookBalance / nativeBalance : 0;
  if (Number.isFinite(derivedRate) && derivedRate > 0) {
    return derivedRate;
  }

  return account.last_exchange_rate && account.last_exchange_rate > 0 ? account.last_exchange_rate : 1;
};

const formatIDRAmount = (amount: number): string => `IDR ${(amount || 0).toLocaleString('id-ID')}`;

const formatRate = (rate: number): string => rate.toLocaleString('id-ID', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 6,
});

const TransferForm: React.FC<TransferFormProps> = ({
  isOpen,
  onClose,
  onSuccess,
  sourceAccount,
}) => {
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    to_account_id: 0,
    exchange_rate: 1,
    reference: '',
    notes: '',
  });

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [accounts, setAccounts] = useState<CashBank[]>([]);
  const [selectedDestAccount, setSelectedDestAccount] = useState<CashBank | null>(null);

  const toast = useToast();
  const { t } = useTranslation();

  useEffect(() => {
    if (!isOpen || !sourceAccount) {
      return;
    }

    const suggestedRate = sourceAccount.last_exchange_rate && sourceAccount.last_exchange_rate > 0
      ? sourceAccount.last_exchange_rate
      : getEffectiveBookRate(sourceAccount);

    setFormData(prev => ({
      ...prev,
      exchange_rate: getCurrencyCode(sourceAccount) === 'IDR' ? 1 : suggestedRate,
    }));
  }, [isOpen, sourceAccount]);

  useEffect(() => {
    const loadAccounts = async () => {
      if (!isOpen || !sourceAccount) {
        return;
      }

      try {
        const allAccounts = await cashbankService.getCashBankAccounts({ includeHolding: true });
        const sourceCurrency = getCurrencyCode(sourceAccount);
        const availableAccounts = allAccounts.filter(acc => {
          if (acc.id === sourceAccount.id || !acc.is_active) {
            return false;
          }

          const destCurrency = getCurrencyCode(acc);
          return destCurrency === sourceCurrency || (sourceCurrency !== 'IDR' && destCurrency === 'IDR');
        });

        setAccounts(availableAccounts);
      } catch (loadError) {
        console.error('Error loading accounts:', loadError);
      }
    };

    loadAccounts();
  }, [isOpen, sourceAccount]);

  const handleInputChange = (field: string, value: string | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
    setError(null);

    if (field === 'to_account_id') {
      const destAccount = accounts.find(acc => acc.id === value);
      setSelectedDestAccount(destAccount || null);

      if (destAccount && sourceAccount) {
        const sourceCurrency = getCurrencyCode(sourceAccount);
        const destCurrency = getCurrencyCode(destAccount);

        if (sourceCurrency !== destCurrency) {
          const suggestedRate = sourceAccount.last_exchange_rate && sourceAccount.last_exchange_rate > 0
            ? sourceAccount.last_exchange_rate
            : getEffectiveBookRate(sourceAccount);
          setFormData(prev => ({
            ...prev,
            exchange_rate: suggestedRate,
          }));
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

      if (!sourceAccount) {
        throw new Error('No source account selected');
      }

      if (!selectedDestAccount) {
        throw new Error('Please select destination account');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      if (formData.amount > getAvailableBalance(sourceAccount)) {
        const frozen = sourceAccount.frozen_amount || 0;
        if (frozen > 0) {
          throw new Error(`Saldo tersedia tidak cukup. Tersedia: ${getAvailableBalance(sourceAccount).toLocaleString('id-ID')} ${sourceAccount.currency} (Total: ${sourceAccount.balance.toLocaleString('id-ID')}, Terblokir: ${frozen.toLocaleString('id-ID')})`);
        }
        throw new Error(`Insufficient balance. Available: ${sourceAccount.currency} ${getAvailableBalance(sourceAccount).toLocaleString('id-ID')}`);
      }

      if (getCurrencyCode(sourceAccount) !== getCurrencyCode(selectedDestAccount) && formData.exchange_rate <= 0) {
        throw new Error('Exchange rate is required for different currencies');
      }

      const requestData: TransferRequest = {
        from_account_id: sourceAccount.id,
        to_account_id: formData.to_account_id,
        date: formData.date,
        amount: formData.amount,
        exchange_rate: formData.exchange_rate,
        reference: formData.reference,
        notes: formData.notes,
      };

      await cashbankService.processTransfer(requestData);

      const destinationAmount = getCurrencyCode(sourceAccount) === getCurrencyCode(selectedDestAccount)
        ? formData.amount
        : formData.amount * formData.exchange_rate;

      toast({
        title: 'Transfer berhasil',
        description: getCurrencyCode(sourceAccount) === getCurrencyCode(selectedDestAccount)
          ? `${formatCashBankAmount(sourceAccount, formData.amount)} dipindahkan dari ${sourceAccount.name} ke ${selectedDestAccount.name}`
          : `${formatCashBankAmount(sourceAccount, formData.amount)} dipindahkan dari ${sourceAccount.name} dan diterima ${formatCashBankAmount(selectedDestAccount, destinationAmount)} di ${selectedDestAccount.name}`,
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      onClose();
    } catch (err: any) {
      console.error('Error processing transfer:', err);
      const message = err.response?.data?.details || err.message || 'Failed to process transfer';
      setError(message);
      toast({
        title: 'Transfer gagal',
        description: message,
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
      notes: '',
    });
    setSelectedDestAccount(null);
    onClose();
  };

  if (!sourceAccount) {
    return null;
  }

  const sourceCurrency = getCurrencyCode(sourceAccount);
  const destinationCurrency = getCurrencyCode(selectedDestAccount);
  const requiresExchangeRate = Boolean(selectedDestAccount && sourceCurrency !== destinationCurrency);
  const effectiveExchangeRate = requiresExchangeRate ? formData.exchange_rate || 0 : 1;
  const convertedAmount = formData.amount * effectiveExchangeRate;
  const sourceBookRate = getEffectiveBookRate(sourceAccount);
  const sourceBookAmountIDR = sourceCurrency === 'IDR'
    ? formData.amount
    : formData.amount * sourceBookRate;
  const destinationBookAmountIDR = selectedDestAccount
    ? (requiresExchangeRate ? convertedAmount : sourceBookAmountIDR)
    : 0;
  const forexDifference = requiresExchangeRate
    ? destinationBookAmountIDR - sourceBookAmountIDR
    : 0;
  const sourceEndingNativeBalance = sourceAccount.balance - formData.amount;
  const sourceEndingBookBalanceIDR = getBookBalanceInIDR(sourceAccount) - sourceBookAmountIDR;
  const destinationEndingNativeBalance = selectedDestAccount
    ? selectedDestAccount.balance + convertedAmount
    : 0;
  const destinationEndingBookBalanceIDR = selectedDestAccount
    ? getBookBalanceInIDR(selectedDestAccount) + destinationBookAmountIDR
    : 0;

  const forexDirection = forexDifference > 0.005
    ? 'gain'
    : forexDifference < -0.005
      ? 'loss'
      : 'neutral';

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="2xl">
      <ModalOverlay />
      <ModalContent>
        <ModalHeader>
          <Flex alignItems="center" gap={3}>
            <Box>
              <Text fontSize="lg" fontWeight="bold">
                {t('cashBank.transferFunds')}
              </Text>
              <Text fontSize="sm" color="gray.500">
                Transfer antar rekening dengan preview nilai buku dan estimasi selisih kurs.
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
                Pilihan rekening
              </Text>

              <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                <Card>
                  <CardBody>
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('cashBank.fromSource')}</Text>
                    <Flex alignItems="center" gap={2} mb={2} wrap="wrap">
                      <Badge colorScheme={sourceAccount.type === 'CASH' ? 'green' : 'blue'} size="sm">
                        {sourceAccount.type}
                      </Badge>
                      {sourceAccount.is_holding_account && (
                        <Badge colorScheme="orange" variant="subtle">HOLDING</Badge>
                      )}
                      {sourceAccount.allow_direct_transactions === false && (
                        <Badge colorScheme="purple" variant="outline">NON OPERASIONAL</Badge>
                      )}
                      <Text fontWeight="medium" fontSize="sm">{sourceAccount.name}</Text>
                    </Flex>

                    <Stat size="sm">
                      <StatLabel>{t('cashBank.availableBalance')}</StatLabel>
                      <StatNumber
                        color={getAvailableBalance(sourceAccount) <= 0 ? 'red.500' : 'green.600'}
                        fontFamily="mono"
                        fontSize="md"
                      >
                        {formatCashBankAmount(sourceAccount, getAvailableBalance(sourceAccount))}
                      </StatNumber>
                      <StatHelpText fontSize="xs">
                        Nilai buku saat ini: {formatIDRAmount(getBookBalanceInIDR(sourceAccount))}
                      </StatHelpText>
                      {sourceCurrency !== 'IDR' && (
                        <StatHelpText fontSize="xs" color="gray.500">
                          Carry rate buku: 1 {sourceCurrency} = {formatRate(sourceBookRate)} IDR
                        </StatHelpText>
                      )}
                      {(sourceAccount.frozen_amount || 0) > 0 && (
                        <StatHelpText color="orange.500" fontSize="xs">
                          Total: {formatCashBankAmount(sourceAccount, sourceAccount.balance)} | Terblokir: {formatCashBankAmount(sourceAccount, sourceAccount.frozen_amount || 0)}
                        </StatHelpText>
                      )}
                    </Stat>
                  </CardBody>
                </Card>

                <Card>
                  <CardBody>
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('cashBank.toDestination')}</Text>
                    <FormControl isRequired>
                      <Select
                        value={formData.to_account_id}
                        onChange={(e) => handleInputChange('to_account_id', parseInt(e.target.value, 10))}
                        placeholder={t('cashBank.selectGLAccountPlaceholder')}
                        size="sm"
                      >
                        {accounts.map((acc) => (
                          <option key={acc.id} value={acc.id}>
                            [{acc.type}] {acc.name} - {getCurrencyCode(acc)} {acc.balance.toLocaleString('id-ID')}
                          </option>
                        ))}
                      </Select>
                    </FormControl>

                    {selectedDestAccount && (
                      <Box mt={3}>
                        <Flex alignItems="center" gap={2} mb={1} wrap="wrap">
                          <Badge colorScheme={selectedDestAccount.type === 'CASH' ? 'green' : 'blue'} size="sm">
                            {selectedDestAccount.type}
                          </Badge>
                          {selectedDestAccount.is_holding_account && (
                            <Badge colorScheme="orange" variant="subtle">HOLDING</Badge>
                          )}
                          {selectedDestAccount.allow_direct_transactions === false && (
                            <Badge colorScheme="purple" variant="outline">NON OPERASIONAL</Badge>
                          )}
                          <Text fontSize="xs" color="gray.600">{selectedDestAccount.name}</Text>
                        </Flex>
                        <Text fontSize="xs" color="gray.500" fontFamily="mono">
                          Saldo saat ini: {formatCashBankAmount(selectedDestAccount, selectedDestAccount.balance)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          Nilai buku saat ini: {formatIDRAmount(getBookBalanceInIDR(selectedDestAccount))}
                        </Text>
                      </Box>
                    )}
                  </CardBody>
                </Card>
              </SimpleGrid>
            </Box>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                Detail transfer
              </Text>

              <VStack spacing={4} align="stretch">
                <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                  <FormControl isRequired>
                    <FormLabel>{t('cashBank.transferDate')}</FormLabel>
                    <Input
                      type="date"
                      value={formData.date}
                      onChange={(e) => handleInputChange('date', e.target.value)}
                    />
                  </FormControl>

                  <FormControl isRequired>
                    <FormLabel>Nominal keluar ({sourceCurrency})</FormLabel>
                    <NumberInput
                      value={formData.amount}
                      onChange={(_, value) => handleInputChange('amount', value || 0)}
                      min={0}
                      precision={2}
                      max={getAvailableBalance(sourceAccount)}
                    >
                      <NumberInputField />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                    {formData.amount > getAvailableBalance(sourceAccount) && (
                      <Text fontSize="xs" color="red.500" mt={1}>
                        Jumlah melebihi saldo tersedia{(sourceAccount.frozen_amount || 0) > 0 ? ` (Dana terblokir: ${formatCashBankAmount(sourceAccount, sourceAccount.frozen_amount || 0)})` : ''}
                      </Text>
                    )}
                  </FormControl>
                </SimpleGrid>

                {requiresExchangeRate && selectedDestAccount && (
                  <Alert status="info" borderRadius="md">
                    <AlertIcon />
                    <Box w="full">
                      <Text fontSize="sm" fontWeight="medium" mb={1}>
                        Kurs transfer {sourceCurrency} ke {destinationCurrency}
                      </Text>
                      <Text fontSize="xs" color="gray.600" mb={3}>
                        Kurs ini menentukan nominal yang diterima rekening tujuan dan perhitungan realized selisih kurs saat jurnal diposting.
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
                          <NumberInputField placeholder="Contoh: 16250" />
                          <NumberInputStepper>
                            <NumberIncrementStepper />
                            <NumberDecrementStepper />
                          </NumberInputStepper>
                        </NumberInput>
                        <Text fontSize="xs" color="gray.500" mt={1}>
                          1 {sourceCurrency} = {formatRate(formData.exchange_rate || 0)} {destinationCurrency}
                        </Text>
                        {sourceCurrency !== 'IDR' && (
                          <Text fontSize="xs" color="gray.500" mt={1}>
                            Nilai buku sumber saat ini: 1 {sourceCurrency} = {formatRate(sourceBookRate)} IDR
                          </Text>
                        )}
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

            {formData.amount > 0 && selectedDestAccount && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  Preview transfer dan jurnal
                </Text>
                <Alert status={requiresExchangeRate ? 'warning' : 'info'} borderRadius="md" alignItems="flex-start">
                  <AlertIcon mt={1} />
                  <VStack align="stretch" spacing={4} w="full">
                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <Box>
                        <Text fontSize="xs" color="gray.600">Rekening sumber</Text>
                        <Text fontSize="sm" fontWeight="bold" color="red.600" fontFamily="mono">
                          -{formatCashBankAmount(sourceAccount, formData.amount)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          Saldo akhir native: {formatCashBankAmount(sourceAccount, sourceEndingNativeBalance)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          Nilai buku akhir: {formatIDRAmount(sourceEndingBookBalanceIDR)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color="gray.600">Rekening tujuan</Text>
                        <Text fontSize="sm" fontWeight="bold" color="green.600" fontFamily="mono">
                          +{formatCashBankAmount(selectedDestAccount, convertedAmount)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          Saldo akhir native: {formatCashBankAmount(selectedDestAccount, destinationEndingNativeBalance)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          Nilai buku akhir: {formatIDRAmount(destinationEndingBookBalanceIDR)}
                        </Text>
                      </Box>
                    </SimpleGrid>

                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <Card variant="outline">
                        <CardBody py={3}>
                          <Text fontSize="xs" color="gray.600">Nilai buku yang dilepas dari sumber</Text>
                          <Text fontWeight="bold" fontFamily="mono" color="red.600">
                            {formatIDRAmount(sourceBookAmountIDR)}
                          </Text>
                          <Text fontSize="xs" color="gray.500" mt={1}>
                            Mengikuti carrying value rekening sumber pada saat transfer diposting.
                          </Text>
                        </CardBody>
                      </Card>

                      <Card variant="outline">
                        <CardBody py={3}>
                          <Text fontSize="xs" color="gray.600">Nilai buku yang masuk ke tujuan</Text>
                          <Text fontWeight="bold" fontFamily="mono" color="green.600">
                            {formatIDRAmount(destinationBookAmountIDR)}
                          </Text>
                          <Text fontSize="xs" color="gray.500" mt={1}>
                            Untuk transfer beda mata uang, angka ini mengikuti nominal tujuan yang dihitung dari kurs transfer.
                          </Text>
                        </CardBody>
                      </Card>
                    </SimpleGrid>

                    {requiresExchangeRate ? (
                      <Box>
                        <Flex alignItems="center" gap={2} mb={1} wrap="wrap">
                          <Text fontSize="xs" color="gray.600">Estimasi selisih kurs terealisasi</Text>
                          {forexDirection === 'gain' && <Badge colorScheme="green">LABA KURS</Badge>}
                          {forexDirection === 'loss' && <Badge colorScheme="red">RUGI KURS</Badge>}
                          {forexDirection === 'neutral' && <Badge colorScheme="gray">NETRAL</Badge>}
                        </Flex>
                        <Text
                          fontSize="sm"
                          fontWeight="bold"
                          fontFamily="mono"
                          color={forexDirection === 'gain' ? 'green.600' : forexDirection === 'loss' ? 'red.500' : 'gray.600'}
                        >
                          {formatIDRAmount(forexDifference)}
                        </Text>
                        <Text fontSize="xs" color="gray.500" mt={1}>
                          Estimasi jurnal: Dr {selectedDestAccount.name} {formatIDRAmount(destinationBookAmountIDR)}, Cr {sourceAccount.name} {formatIDRAmount(sourceBookAmountIDR)}
                          {forexDirection === 'gain' ? `, Cr Laba Selisih Kurs ${formatIDRAmount(forexDifference)}` : ''}
                          {forexDirection === 'loss' ? `, Dr Rugi Selisih Kurs ${formatIDRAmount(Math.abs(forexDifference))}` : ''}
                        </Text>
                      </Box>
                    ) : (
                      <Text fontSize="xs" color="gray.500">
                        Tidak ada selisih kurs karena rekening sumber dan tujuan memakai mata uang yang sama.
                      </Text>
                    )}

                    <Text fontSize="xs" color="gray.500">
                      Preview ini mengikuti logika posting backend saat ini. Nilai final akan memakai kurs dan nominal yang Anda simpan saat submit.
                    </Text>
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
              formData.amount > getAvailableBalance(sourceAccount) ||
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
