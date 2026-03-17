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
  Card,
  CardBody,
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
  useToast,
  VStack,
} from '@chakra-ui/react';
import cashbankService, {
  CashBank,
  formatCashBankAmount,
  getAvailableBalance,
  getBookBalanceInIDR,
  ReferenceExchangeRate,
  TransferRequest,
} from '@/services/cashbankService';
import { useTranslation } from '@/hooks/useTranslation';
import { roundUsdIdrRate } from '@/utils/exchangeRate';

interface TransferFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  sourceAccount: CashBank | null;
}

interface ApiErrorShape {
  response?: {
    data?: {
      details?: string;
      message?: string;
    };
  };
  message?: string;
}

const getCurrencyCode = (account?: CashBank | null): string => (account?.currency || 'IDR').toUpperCase();

const formatIDRAmount = (amount: number): string => `IDR ${(amount || 0).toLocaleString('id-ID', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 2,
})}`;

const formatRate = (rate: number): string => rate.toLocaleString('id-ID', {
  minimumFractionDigits: 2,
  maximumFractionDigits: 6,
});

const formatRoundedRate = (rate: number): string => roundUsdIdrRate(rate, 0).toLocaleString('id-ID', {
  minimumFractionDigits: 0,
  maximumFractionDigits: 0,
});

const formatReferenceTimestamp = (value?: string): string => {
  if (!value) {
    return '-';
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return value;
  }

  return parsed.toLocaleString('id-ID');
};

const getTransferRateSourceMeta = (
  t: (key: string, params?: Record<string, string | number>) => string,
  options: {
  hasReferenceRate: boolean;
  hasLastRate: boolean;
  usesCarryRate: boolean;
  isManualOverride: boolean;
}) => {
  if (options.isManualOverride) {
    return {
      label: t('cashBank.rateSourceManualLabel'),
      colorScheme: 'orange',
      description: t('cashBank.rateSourceManualDesc'),
    };
  }

  if (options.hasReferenceRate) {
    return {
      label: t('cashBank.rateSourceApiLabel'),
      colorScheme: 'blue',
      description: t('cashBank.rateSourceApiDesc'),
    };
  }

  if (options.hasLastRate) {
    return {
      label: t('cashBank.rateSourceLastRateLabel'),
      colorScheme: 'purple',
      description: t('cashBank.rateSourceLastRateDesc'),
    };
  }

  if (options.usesCarryRate) {
    return {
      label: t('cashBank.rateSourceCarryRateLabel'),
      colorScheme: 'teal',
      description: t('cashBank.rateSourceCarryRateDesc'),
    };
  }

  return {
    label: t('cashBank.rateSourceManualLabel'),
    colorScheme: 'gray',
    description: t('cashBank.rateSourceAwaitingManualDesc'),
  };
};

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

const supportsUsdIdrPair = (sourceCurrency: string, destinationCurrency: string): boolean => {
  if (sourceCurrency === destinationCurrency) {
    return true;
  }

  return (sourceCurrency === 'USD' && destinationCurrency === 'IDR')
    || (sourceCurrency === 'IDR' && destinationCurrency === 'USD');
};

const getSuggestedCrossRate = (sourceAccount: CashBank, destinationAccount?: CashBank | null): number => {
  const sourceCurrency = getCurrencyCode(sourceAccount);
  const destinationCurrency = getCurrencyCode(destinationAccount);

  if (sourceCurrency === destinationCurrency) {
    return 1;
  }

  const foreignAccount = sourceCurrency === 'USD' ? sourceAccount : destinationAccount;
  if (foreignAccount?.last_exchange_rate && foreignAccount.last_exchange_rate > 0) {
    return roundUsdIdrRate(foreignAccount.last_exchange_rate, 1);
  }

  return foreignAccount ? roundUsdIdrRate(getEffectiveBookRate(foreignAccount), 1) : 1;
};

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
  const [referenceRateInfo, setReferenceRateInfo] = useState<ReferenceExchangeRate | null>(null);
  const [referenceRateError, setReferenceRateError] = useState<string | null>(null);
  const [loadingReferenceRate, setLoadingReferenceRate] = useState(false);

  const toast = useToast();
  const { t } = useTranslation();

  useEffect(() => {
    if (!isOpen || !sourceAccount) {
      return;
    }

    setFormData(prev => ({
      ...prev,
      exchange_rate: getSuggestedCrossRate(sourceAccount, selectedDestAccount),
    }));
  }, [isOpen, sourceAccount, selectedDestAccount]);

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

          const destinationCurrency = getCurrencyCode(acc);
          return supportsUsdIdrPair(sourceCurrency, destinationCurrency);
        });

        setAccounts(availableAccounts);
      } catch (loadError) {
        console.error('Error loading accounts:', loadError);
      }
    };

    void loadAccounts();
  }, [isOpen, sourceAccount]);

  useEffect(() => {
    if (!isOpen || !sourceAccount || !selectedDestAccount) {
      setReferenceRateInfo(null);
      setReferenceRateError(null);
      setLoadingReferenceRate(false);
      return;
    }

    const sourceCurrency = getCurrencyCode(sourceAccount);
    const destinationCurrency = getCurrencyCode(selectedDestAccount);
    if (sourceCurrency === destinationCurrency) {
      setReferenceRateInfo(null);
      setReferenceRateError(null);
      setLoadingReferenceRate(false);
      return;
    }

    let cancelled = false;

    const loadReferenceRate = async () => {
      try {
        setLoadingReferenceRate(true);
        setReferenceRateError(null);
        const reference = await cashbankService.getReferenceExchangeRate('USD', 'IDR');
        if (cancelled) {
          return;
        }
        if (!reference) {
          setReferenceRateInfo(null);
          setReferenceRateError(t('cashBank.referenceRateUnavailable'));
          return;
        }

        setReferenceRateInfo(reference);
        setFormData(prev => ({
          ...prev,
          exchange_rate: roundUsdIdrRate(reference.exchange_rate, 1),
        }));
      } catch (fetchError) {
        if (cancelled) {
          return;
        }

        console.error('Error loading USD/IDR reference rate for transfer:', fetchError);
        setReferenceRateInfo(null);
        setReferenceRateError(t('cashBank.referenceRateUnavailable'));
      } finally {
        if (!cancelled) {
          setLoadingReferenceRate(false);
        }
      }
    };

    void loadReferenceRate();

    return () => {
      cancelled = true;
    };
  }, [isOpen, sourceAccount, selectedDestAccount, t]);

  const handleInputChange = (field: string, value: string | number) => {
    setFormData(prev => ({
      ...prev,
      [field]: value,
    }));
    setError(null);

    if (field === 'to_account_id') {
      const destinationAccount = accounts.find(acc => acc.id === value);
      setSelectedDestAccount(destinationAccount || null);
      if (destinationAccount && sourceAccount) {
        setFormData(prev => ({
          ...prev,
          exchange_rate: getSuggestedCrossRate(sourceAccount, destinationAccount),
        }));
      }
    }
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      if (!sourceAccount) {
        throw new Error(t('cashBank.sourceAccountRequired'));
      }

      if (!selectedDestAccount) {
        throw new Error(t('cashBank.destinationAccountRequired'));
      }

      if (formData.amount <= 0) {
        throw new Error(t('cashBank.amountMustBeGreaterThanZero'));
      }

      if (formData.amount > getAvailableBalance(sourceAccount)) {
        const frozen = sourceAccount.frozen_amount || 0;
        if (frozen > 0) {
          throw new Error(t('cashBank.insufficientAvailableBalanceDetailed', {
            available: getAvailableBalance(sourceAccount).toLocaleString('id-ID'),
            currency: sourceAccount.currency,
            total: sourceAccount.balance.toLocaleString('id-ID'),
            frozen: frozen.toLocaleString('id-ID'),
          }));
        }
        throw new Error(t('cashBank.insufficientAvailableBalance', {
          currency: sourceAccount.currency,
          available: getAvailableBalance(sourceAccount).toLocaleString('id-ID'),
        }));
      }

      if (requiresExchangeRate && formData.exchange_rate <= 0) {
        throw new Error(t('cashBank.exchangeRateRequiredUsdIdr'));
      }

      const requestData: TransferRequest = {
        from_account_id: sourceAccount.id,
        to_account_id: formData.to_account_id,
        date: formData.date,
        amount: formData.amount,
        exchange_rate: requiresExchangeRate ? roundUsdIdrRate(formData.exchange_rate, 1) : formData.exchange_rate,
        reference: formData.reference,
        notes: formData.notes,
      };

      await cashbankService.processTransfer(requestData);

      toast({
        title: t('cashBank.transferSuccessful'),
        description: requiresExchangeRate
          ? t('cashBank.transferFxSuccessDesc', {
            sourceAmount: formatCashBankAmount(sourceAccount, formData.amount),
            sourceName: sourceAccount.name,
            destinationAmount: formatCashBankAmount(selectedDestAccount, destinationNativeAmount),
            destinationName: selectedDestAccount.name,
          })
          : t('cashBank.transferSuccessDesc', {
            sourceAmount: formatCashBankAmount(sourceAccount, formData.amount),
            sourceName: sourceAccount.name,
            destinationName: selectedDestAccount.name,
          }),
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      onClose();
    } catch (submitError: unknown) {
      console.error('Error processing transfer:', submitError);
      const normalizedError = submitError as ApiErrorShape;
      const message = normalizedError.response?.data?.details || normalizedError.message || t('cashBank.transferProcessFailed');
      setError(message);
      toast({
        title: t('cashBank.transactionFailed'),
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
  const sourceToIdr = sourceCurrency === 'USD' && destinationCurrency === 'IDR';
  const effectiveExchangeRate = requiresExchangeRate ? formData.exchange_rate || 0 : 1;
  const destinationNativeAmount = !selectedDestAccount
    ? 0
    : !requiresExchangeRate
      ? formData.amount
      : sourceToIdr
        ? formData.amount * effectiveExchangeRate
        : formData.amount / effectiveExchangeRate;

  const sourceBookRate = getEffectiveBookRate(sourceAccount);
  const sourceBookAmountIDR = sourceCurrency === 'IDR'
    ? formData.amount
    : formData.amount * sourceBookRate;
  const destinationBookAmountIDR = !selectedDestAccount
    ? 0
    : !requiresExchangeRate
      ? sourceBookAmountIDR
      : sourceCurrency === 'IDR'
        ? formData.amount
        : destinationNativeAmount;
  const forexDifference = sourceToIdr ? destinationBookAmountIDR - sourceBookAmountIDR : 0;
  const sourceEndingNativeBalance = sourceAccount.balance - formData.amount;
  const sourceEndingBookBalanceIDR = getBookBalanceInIDR(sourceAccount) - sourceBookAmountIDR;
  const destinationEndingNativeBalance = selectedDestAccount
    ? selectedDestAccount.balance + destinationNativeAmount
    : 0;
  const destinationEndingBookBalanceIDR = selectedDestAccount
    ? getBookBalanceInIDR(selectedDestAccount) + destinationBookAmountIDR
    : 0;
  const transferDirection = selectedDestAccount ? `${sourceCurrency} -> ${destinationCurrency}` : `${sourceCurrency} -> ?`;
  const roundedReferenceRate = roundUsdIdrRate(referenceRateInfo?.exchange_rate, 0);
  const exchangeRateOverridden = Boolean(referenceRateInfo && Math.abs((formData.exchange_rate || 0) - roundedReferenceRate) > 0.0001);
  const foreignRateAccount = sourceCurrency === 'USD' ? sourceAccount : selectedDestAccount;
  const transferRateSource = getTransferRateSourceMeta(t, {
    hasReferenceRate: Boolean(referenceRateInfo),
    hasLastRate: Boolean(foreignRateAccount?.last_exchange_rate && foreignRateAccount.last_exchange_rate > 0),
    usesCarryRate: Boolean(foreignRateAccount && (!foreignRateAccount.last_exchange_rate || foreignRateAccount.last_exchange_rate <= 0)),
    isManualOverride: exchangeRateOverridden,
  });
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
          <Box>
            <Text fontSize="lg" fontWeight="bold">
              {t('cashBank.transferFunds')}
            </Text>
            <Text fontSize="sm" color="gray.500">
              {t('cashBank.transferUsdIdrHelper')}
            </Text>
          </Box>
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
                {t('cashBank.accountSelection')}
              </Text>

              <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                <Card>
                  <CardBody>
                    <Text fontSize="sm" color="gray.600" mb={2}>{t('cashBank.fromSource')}</Text>
                    <Flex alignItems="center" gap={2} mb={2} wrap="wrap">
                      <Badge colorScheme={sourceAccount.type === 'CASH' ? 'green' : 'blue'} size="sm">
                        {sourceAccount.type}
                      </Badge>
                      <Text fontWeight="medium" fontSize="sm">{sourceAccount.name}</Text>
                    </Flex>

                    <Stat size="sm">
                      <StatLabel>{t('cashBank.availableBalance')}</StatLabel>
                      <StatNumber color={getAvailableBalance(sourceAccount) <= 0 ? 'red.500' : 'green.600'} fontFamily="mono" fontSize="md">
                        {formatCashBankAmount(sourceAccount, getAvailableBalance(sourceAccount))}
                      </StatNumber>
                      <StatHelpText fontSize="xs">
                        {t('cashBank.currentBookValue')}: {formatIDRAmount(getBookBalanceInIDR(sourceAccount))}
                      </StatHelpText>
                      {sourceCurrency === 'USD' && (
                        <StatHelpText fontSize="xs" color="gray.500">
                          {t('cashBank.carryBookRate')}: 1 USD = {formatRate(sourceBookRate)} IDR
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
                        placeholder={t('cashBank.selectDestinationAccountPlaceholder')}
                        size="sm"
                      >
                        {accounts.map(acc => (
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
                          <Text fontSize="xs" color="gray.600">{selectedDestAccount.name}</Text>
                        </Flex>
                        <Text fontSize="xs" color="gray.500" fontFamily="mono">
                          {t('cashBank.currentBalance')}: {formatCashBankAmount(selectedDestAccount, selectedDestAccount.balance)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {t('cashBank.currentBookValue')}: {formatIDRAmount(getBookBalanceInIDR(selectedDestAccount))}
                        </Text>
                      </Box>
                    )}
                  </CardBody>
                </Card>
              </SimpleGrid>
            </Box>

            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                {t('cashBank.transferDetails')}
              </Text>

              {selectedDestAccount && (
                <Alert status={requiresExchangeRate ? 'info' : 'success'} borderRadius="md" mb={4} alignItems="flex-start">
                  <AlertIcon mt={1} />
                  <Box>
                    <Flex gap={2} mb={2} wrap="wrap">
                      <Badge colorScheme="blue" variant="subtle">{transferDirection}</Badge>
                      <Badge colorScheme="green" variant="subtle">BOOK IDR</Badge>
                    </Flex>
                    <Text fontSize="sm" fontWeight="medium">
                      {requiresExchangeRate
                        ? t('cashBank.fxTransferImpact')
                        : t('cashBank.sameCurrencyTransferImpact')}
                    </Text>
                    <Text fontSize="xs" color="gray.600" mt={1}>
                      {sourceToIdr
                        ? t('cashBank.usdToIdrTransferExplanation')
                        : requiresExchangeRate
                          ? t('cashBank.idrToUsdTransferExplanation')
                          : t('cashBank.sameCurrencyTransferExplanation')}
                    </Text>
                  </Box>
                </Alert>
              )}

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
                    <FormLabel>{t('cashBank.outgoingAmountLabel', { currency: sourceCurrency })}</FormLabel>
                    <NumberInput
                      value={formData.amount}
                      onChange={(_, value) => handleInputChange('amount', value || 0)}
                      min={0}
                      precision={sourceCurrency === 'USD' ? 2 : 0}
                      max={getAvailableBalance(sourceAccount)}
                    >
                      <NumberInputField fontFamily="mono" />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                    {formData.amount > getAvailableBalance(sourceAccount) && (
                      <Text fontSize="xs" color="red.500" mt={1}>
                        {t('cashBank.amountExceedsBalance')}
                      </Text>
                    )}
                  </FormControl>
                </SimpleGrid>

                {requiresExchangeRate && selectedDestAccount && (
                  <Alert status="info" borderRadius="md">
                    <AlertIcon />
                    <Box w="full">
                      <Text fontSize="sm" fontWeight="medium" mb={1}>
                        {t('cashBank.transferRateTitle')}
                      </Text>
                      <Text fontSize="xs" color="gray.600" mb={3}>
                        {t('cashBank.transferRateDesc')}
                      </Text>
                      <FormControl>
                        <FormLabel fontSize="sm">{t('cashBank.exchangeRate')}</FormLabel>
                        <NumberInput
                          value={formData.exchange_rate}
                          onChange={(_, value) => handleInputChange('exchange_rate', roundUsdIdrRate(value, 1))}
                          min={0}
                          precision={0}
                          step={1}
                        >
                          <NumberInputField placeholder={t('cashBank.exchangeRatePlaceholder')} fontFamily="mono" />
                          <NumberInputStepper>
                            <NumberIncrementStepper />
                            <NumberDecrementStepper />
                          </NumberInputStepper>
                        </NumberInput>
                        <Text fontSize="xs" color="gray.500" mt={1}>
                          1 USD = {formatRoundedRate(formData.exchange_rate || 0)} IDR
                        </Text>
                        {referenceRateInfo && (
                          <Alert status="info" borderRadius="md" mt={3}>
                            <AlertIcon />
                            <Box>
                              <AlertTitle fontSize="sm" mb={1}>
                                {t('cashBank.apiReferenceRate')}
                              </AlertTitle>
                              <AlertDescription fontSize="sm">
                                {t('cashBank.apiReferenceRateDesc', {
                                  provider: referenceRateInfo.provider,
                                  rate: formatRoundedRate(referenceRateInfo.exchange_rate),
                                  fetchedAt: formatReferenceTimestamp(referenceRateInfo.fetched_at),
                                })}
                              </AlertDescription>
                            </Box>
                          </Alert>
                        )}
                        {loadingReferenceRate && (
                          <Text fontSize="xs" color="gray.500" mt={2}>
                            {t('cashBank.loadingReferenceRate')}
                          </Text>
                        )}
                        {referenceRateError && (
                          <Alert status="warning" borderRadius="md" mt={3}>
                            <AlertIcon />
                            <AlertDescription fontSize="sm">
                              {referenceRateError}
                            </AlertDescription>
                          </Alert>
                        )}
                        {exchangeRateOverridden && (
                          <Text fontSize="xs" color="orange.500" mt={2}>
                            {t('cashBank.manualRateOverrideWarning')}
                          </Text>
                        )}
                        <HStack spacing={2} mt={2} align="center">
                          <Badge colorScheme={transferRateSource.colorScheme} variant="subtle">
                            {transferRateSource.label}
                          </Badge>
                          <Text fontSize="xs" color="gray.500">
                            {transferRateSource.description}
                          </Text>
                        </HStack>
                        <Text fontSize="xs" color="gray.500" mt={1}>
                          {sourceToIdr
                            ? t('cashBank.destinationFormulaMultiply', { sourceCurrency, destinationCurrency })
                            : t('cashBank.destinationFormulaDivide', { sourceCurrency, destinationCurrency })}
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
                    placeholder={t('common.placeholders.referenceNumber')}
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>{t('payments.form.notes')}</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={(e) => handleInputChange('notes', e.target.value)}
                    placeholder={t('common.placeholders.notes')}
                    rows={3}
                  />
                </FormControl>
              </VStack>
            </Box>

            {formData.amount > 0 && selectedDestAccount && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  {t('cashBank.transferPreviewAndJournal')}
                </Text>
                <Alert status={requiresExchangeRate ? 'warning' : 'info'} borderRadius="md" alignItems="flex-start">
                  <AlertIcon mt={1} />
                  <VStack align="stretch" spacing={4} w="full">
                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <Box>
                        <Text fontSize="xs" color="gray.600">{t('cashBank.sourceAccountNativeOut')}</Text>
                        <Text fontSize="sm" fontWeight="bold" color="red.600" fontFamily="mono">
                          -{formatCashBankAmount(sourceAccount, formData.amount)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {t('cashBank.endingNativeBalance')}: {formatCashBankAmount(sourceAccount, sourceEndingNativeBalance)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {t('cashBank.endingBookValue')}: {formatIDRAmount(sourceEndingBookBalanceIDR)}
                        </Text>
                      </Box>
                      <Box>
                        <Text fontSize="xs" color="gray.600">{t('cashBank.destinationAccountNativeIn')}</Text>
                        <Text fontSize="sm" fontWeight="bold" color="green.600" fontFamily="mono">
                          +{formatCashBankAmount(selectedDestAccount, destinationNativeAmount)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {t('cashBank.endingNativeBalance')}: {formatCashBankAmount(selectedDestAccount, destinationEndingNativeBalance)}
                        </Text>
                        <Text fontSize="xs" color="gray.500">
                          {t('cashBank.endingBookValue')}: {formatIDRAmount(destinationEndingBookBalanceIDR)}
                        </Text>
                      </Box>
                    </SimpleGrid>

                    <SimpleGrid columns={{ base: 1, md: 2 }} spacing={4}>
                      <Card variant="outline">
                        <CardBody py={3}>
                          <Text fontSize="xs" color="gray.600">{t('cashBank.sourceBookAmountReleased')}</Text>
                          <Text fontWeight="bold" fontFamily="mono" color="red.600">
                            {formatIDRAmount(sourceBookAmountIDR)}
                          </Text>
                          <Text fontSize="xs" color="gray.500" mt={1}>
                            {t('cashBank.sourceBookAmountReleasedHelp')}
                          </Text>
                        </CardBody>
                      </Card>

                      <Card variant="outline">
                        <CardBody py={3}>
                          <Text fontSize="xs" color="gray.600">{t('cashBank.destinationBookAmountPosted')}</Text>
                          <Text fontWeight="bold" fontFamily="mono" color="green.600">
                            {formatIDRAmount(destinationBookAmountIDR)}
                          </Text>
                          <Text fontSize="xs" color="gray.500" mt={1}>
                            {t('cashBank.destinationBookAmountPostedHelp')}
                          </Text>
                        </CardBody>
                      </Card>
                    </SimpleGrid>

                    {requiresExchangeRate ? (
                      <Box>
                        <Flex alignItems="center" gap={2} mb={1} wrap="wrap">
                          <Text fontSize="xs" color="gray.600">{t('cashBank.estimatedRealizedFx')}</Text>
                          {forexDirection === 'gain' && <Badge colorScheme="green">{t('cashBank.fxGainBadge')}</Badge>}
                          {forexDirection === 'loss' && <Badge colorScheme="red">{t('cashBank.fxLossBadge')}</Badge>}
                          {forexDirection === 'neutral' && <Badge colorScheme="gray">{t('cashBank.fxNeutralBadge')}</Badge>}
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
                          {t('cashBank.estimatedJournalEntry', {
                            destinationName: selectedDestAccount.name,
                            destinationBookAmount: formatIDRAmount(destinationBookAmountIDR),
                            sourceName: sourceAccount.name,
                            sourceBookAmount: formatIDRAmount(sourceBookAmountIDR),
                          })}
                          {forexDirection === 'gain' ? `, ${t('cashBank.estimatedJournalGain', { amount: formatIDRAmount(forexDifference) })}` : ''}
                          {forexDirection === 'loss' ? `, ${t('cashBank.estimatedJournalLoss', { amount: formatIDRAmount(Math.abs(forexDifference)) })}` : ''}
                        </Text>
                      </Box>
                    ) : (
                      <Box>
                        <Text fontSize="xs" color="gray.500">
                          {!requiresExchangeRate
                            ? t('cashBank.noFxDifferenceSameCurrency')
                            : t('cashBank.noRealizedFxOnUsdPurchase')}
                        </Text>
                      </Box>
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
              formData.amount <= 0
              || !selectedDestAccount
              || formData.amount > getAvailableBalance(sourceAccount)
              || (requiresExchangeRate && formData.exchange_rate <= 0)
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
