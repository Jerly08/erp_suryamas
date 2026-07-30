'use client';

import React, { useEffect, useMemo, useState } from 'react';
import {
  Alert,
  AlertIcon,
  Box,
  Button,
  HStack,
  Input,
  InputGroup,
  InputLeftElement,
  Modal,
  ModalBody,
  ModalCloseButton,
  ModalContent,
  ModalFooter,
  ModalHeader,
  ModalOverlay,
  Spinner,
  Text,
  useColorModeValue,
  useToast,
  VStack,
} from '@chakra-ui/react';
import { FiArrowRight, FiSearch } from 'react-icons/fi';
import PurchasePaymentForm from '@/components/purchase/PurchasePaymentForm';
import { Purchase } from '@/services/purchaseService';
import purchaseService from '@/services/purchaseService';
import cashbankService, { CashBank } from '@/services/cashbankService';
import { useTranslation } from '@/hooks/useTranslation';

interface PurchaseTermPaymentPickerProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess?: () => void;
}

const OPEN_BALANCE_TOLERANCE = 0.005;

const formatCurrency = (amount: number) =>
  new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: Math.abs(amount % 1) > Number.EPSILON ? 2 : 0,
    maximumFractionDigits: 2,
  }).format(amount || 0);

const normalizeSearchValue = (value?: string | number | null) =>
  String(value ?? '').trim().toLocaleLowerCase('id-ID');

const PurchaseTermPaymentPicker: React.FC<PurchaseTermPaymentPickerProps> = ({
  isOpen,
  onClose,
  onSuccess,
}) => {
  const { t } = useTranslation();
  const toast = useToast();
  const [search, setSearch] = useState('');
  const [purchases, setPurchases] = useState<Purchase[]>([]);
  const [cashBanks, setCashBanks] = useState<CashBank[]>([]);
  const [loading, setLoading] = useState(false);
  const [selectedPurchase, setSelectedPurchase] = useState<Purchase | null>(null);

  const modalBg = useColorModeValue('white', 'gray.800');
  const rowBg = useColorModeValue('white', 'gray.750');
  const mutedText = useColorModeValue('gray.600', 'gray.400');
  const borderColor = useColorModeValue('gray.200', 'whiteAlpha.300');

  useEffect(() => {
    if (!isOpen || selectedPurchase) {
      return;
    }

    let isCurrent = true;
    const timer = window.setTimeout(async () => {
      setLoading(true);
      try {
        const [purchaseResult, accounts] = await Promise.all([
          purchaseService.list({
            outstanding: true,
            search: search.trim() || undefined,
            page: 1,
            limit: 100,
          }),
          cashbankService.getPaymentAccounts(),
        ]);

        if (!isCurrent) return;

        setPurchases(
          purchaseResult.data.filter(
            (purchase) =>
              purchase.payment_method?.toUpperCase() === 'CREDIT' &&
              Number(purchase.outstanding_amount || 0) > OPEN_BALANCE_TOLERANCE,
          ),
        );
        setCashBanks(accounts);
      } catch (error) {
        console.error('Failed to load outstanding purchase terms:', error);
        if (!isCurrent) return;
        toast({
          title: t('common.error'),
          description: t('payments.purchaseTermLoadError'),
          status: 'error',
          duration: 4000,
          isClosable: true,
        });
      } finally {
        if (isCurrent) setLoading(false);
      }
    }, search.trim() ? 250 : 0);

    return () => {
      isCurrent = false;
      window.clearTimeout(timer);
    };
  }, [isOpen, search, selectedPurchase, t, toast]);

  const visiblePurchases = useMemo(() => {
    const normalizedSearch = normalizeSearchValue(search);
    if (!normalizedSearch) return purchases;

    return purchases.filter((purchase) => {
      const searchableValues = [
        purchase.code,
        purchase.vendor?.name,
        purchase.vendor?.code,
        purchase.purchase_request?.pr_number,
        purchase.purchase_request?.request_number,
        purchase.purchase_request?.title,
        purchase.project?.code,
        purchase.project?.name,
      ];
      return searchableValues.some((value) =>
        normalizeSearchValue(value).includes(normalizedSearch),
      );
    });
  }, [purchases, search]);

  // PurchasePaymentForm predates the shared cash-bank service type and expects
  // `account_code`. Keep the adapter here so both payment flows use the same
  // operational cash/bank list without widening the older component contract.
  const purchasePaymentCashBanks = useMemo(
    () =>
      cashBanks.map((account) => ({
        id: account.id,
        name: account.name,
        account_code: account.code || account.account?.code || '',
        balance: account.balance,
        currency: account.currency,
        book_balance_idr: account.book_balance_idr,
        last_exchange_rate: account.last_exchange_rate,
        type: account.type,
      })),
    [cashBanks],
  );

  const handleClose = () => {
    setSearch('');
    setSelectedPurchase(null);
    onClose();
  };

  const handleSelectPurchase = async (purchase: Purchase) => {
    setLoading(true);
    try {
      const latestPurchase = await purchaseService.getById(purchase.id);
      if (Number(latestPurchase.outstanding_amount || 0) <= OPEN_BALANCE_TOLERANCE) {
        toast({
          title: t('messages.warning'),
          description: t('payments.purchaseTermNoOutstanding'),
          status: 'warning',
          duration: 3500,
          isClosable: true,
        });
        return;
      }
      setSelectedPurchase(latestPurchase);
    } catch (error) {
      console.error('Failed to load selected purchase:', error);
      toast({
        title: t('common.error'),
        description: t('payments.purchaseTermLoadError'),
        status: 'error',
        duration: 4000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  const termCount = (purchase: Purchase) => purchase.payment_terms?.length || 0;

  return (
    <>
      <Modal isOpen={isOpen && !selectedPurchase} onClose={handleClose} size="xl" scrollBehavior="inside">
        <ModalOverlay />
        <ModalContent maxH="90vh" bg={modalBg}>
          <ModalHeader>{t('payments.purchaseTermPayment')}</ModalHeader>
          <ModalCloseButton />
          <ModalBody overflowY="auto">
            <VStack align="stretch" spacing={4}>
              <Alert status="info" borderRadius="md">
                <AlertIcon />
                <Text fontSize="sm">{t('payments.purchaseTermPaymentDescription')}</Text>
              </Alert>

              <InputGroup>
                <InputLeftElement pointerEvents="none">
                  <FiSearch />
                </InputLeftElement>
                <Input
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder={t('payments.purchaseTermSearchPlaceholder')}
                />
              </InputGroup>

              {loading ? (
                <HStack justify="center" py={8} spacing={3}>
                  <Spinner />
                  <Text color={mutedText}>{t('common.loading')}</Text>
                </HStack>
              ) : visiblePurchases.length === 0 ? (
                <Box py={8} textAlign="center" borderTopWidth="1px" borderColor={borderColor}>
                  <Text color={mutedText}>{t('payments.purchaseTermEmpty')}</Text>
                </Box>
              ) : (
                <VStack align="stretch" spacing={2}>
                  {visiblePurchases.map((purchase) => (
                    <Box
                      key={purchase.id}
                      borderWidth="1px"
                      borderColor={borderColor}
                      borderRadius="md"
                      p={4}
                      bg={rowBg}
                    >
                      <HStack justify="space-between" align="start" spacing={4}>
                        <Box minW={0}>
                          <Text fontWeight="semibold">{purchase.code}</Text>
                          <Text fontSize="sm" noOfLines={1}>{purchase.vendor?.name || '-'}</Text>
                          <Text fontSize="xs" color={mutedText} mt={1} noOfLines={1}>
                            {purchase.purchase_request?.pr_number || purchase.purchase_request?.request_number || '-'}
                            {purchase.project?.code ? ` | ${purchase.project.code}` : ''}
                          </Text>
                          <Text fontSize="xs" color={mutedText} mt={1}>
                            {termCount(purchase) > 0
                              ? t('payments.purchaseTermCount', { count: termCount(purchase) })
                              : t('payments.purchaseTermNoSchedule')}
                          </Text>
                        </Box>
                        <VStack align="end" spacing={2} flexShrink={0}>
                          <Text fontWeight="semibold" color="orange.500">
                            {formatCurrency(Number(purchase.outstanding_amount || 0))}
                          </Text>
                          <Button
                            size="sm"
                            colorScheme="blue"
                            rightIcon={<FiArrowRight />}
                            onClick={() => handleSelectPurchase(purchase)}
                          >
                            {t('payments.selectPurchaseTerm')}
                          </Button>
                        </VStack>
                      </HStack>
                    </Box>
                  ))}
                </VStack>
              )}
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" onClick={handleClose}>{t('common.cancel')}</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      <PurchasePaymentForm
        isOpen={isOpen && Boolean(selectedPurchase)}
        onClose={handleClose}
        purchase={selectedPurchase}
        cashBanks={purchasePaymentCashBanks}
        title={t('payments.purchaseTermPayment')}
        onSuccess={() => {
          onSuccess?.();
          handleClose();
        }}
      />
    </>
  );
};

export default PurchaseTermPaymentPicker;
