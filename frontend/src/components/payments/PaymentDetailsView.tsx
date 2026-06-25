'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  Card,
  CardHeader,
  CardBody,
  Heading,
  Text,
  VStack,
  HStack,
  Badge,
  Divider,
  Button,
  Icon,
  useToast,
  Alert,
  AlertIcon,
  Flex,
  Spacer,
  Tag,
  TagLabel,
  TagLeftIcon,
  useColorModeValue,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
} from '@chakra-ui/react';
import {
  FiDollarSign,
  FiFileText,
  FiRefreshCw,
  FiArrowRight,
  FiArrowDown,
  FiExternalLink,
  FiRotateCcw,
} from 'react-icons/fi';
import paymentService from '@/services/paymentService';
import JournalEntryPreview from './JournalEntryPreview';
import type {
  PaymentWithJournalResponse,
  AccountBalanceUpdate,
  JournalEntry,
  PaymentJournalResult,
} from '@/services/paymentService';

interface PaymentDetailsViewProps {
  paymentId: string;
  onEdit?: (paymentId: string) => void;
  onReverse?: (paymentId: string) => void;
}

// Currency formatter
const formatCurrency = (amount: number) => {
  const hasFraction = Math.abs((amount || 0) % 1) > Number.EPSILON;

  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: hasFraction ? 2 : 0,
    maximumFractionDigits: 2
  }).format(amount);
};

const PaymentDetailsView: React.FC<PaymentDetailsViewProps> = ({
  paymentId,
  onEdit,
  onReverse
}) => {
  const [paymentInfo, setPaymentInfo] = useState<PaymentWithJournalResponse | null>(null);
  const [accountBalances, setAccountBalances] = useState<AccountBalanceUpdate[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [isReversing, setIsReversing] = useState(false);
  const [reverseResult, setReverseResult] = useState<PaymentJournalResult | null>(null);

  const { isOpen: isReverseModalOpen, onOpen: onReverseModalOpen, onClose: onReverseModalClose } = useDisclosure();
  const toast = useToast();

  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const textSecondary = useColorModeValue('gray.600', 'gray.400');
  const textPrimary = useColorModeValue('gray.700', 'gray.200');
  const numericPaymentId = Number(paymentId);

  useEffect(() => {
    loadPaymentDetails();
  }, [paymentId]);

  const loadPaymentDetails = async () => {
    setIsLoading(true);
    try {
      const details = await paymentService.getPaymentWithJournal(numericPaymentId);
      setPaymentInfo(details);
      
      if (details.journal_result?.account_updates?.length) {
        setAccountBalances(details.journal_result.account_updates);
      } else if (details.journal_result?.journal_entry?.id) {
        try {
          const balances = await paymentService.getAccountBalanceUpdates(numericPaymentId);
          setAccountBalances(balances);
        } catch (error) {
          console.warn('Failed to load account balances:', error);
        }
      }
    } catch (error: any) {
      toast({
        title: 'Load Error',
        description: error.message || 'Failed to load payment details',
        status: 'error',
        duration: 5000,
        isClosable: true
      });
    } finally {
      setIsLoading(false);
    }
  };

  const handleReversePayment = async () => {
    setIsReversing(true);
    try {
      const result = await paymentService.reversePayment(numericPaymentId, 'Manual reversal from UI');
      
      setReverseResult(result.journal_result ?? null);
      
      toast({
        title: 'Payment Reversed',
        description: 'Payment and journal entries have been reversed successfully',
        status: 'success',
        duration: 5000,
        isClosable: true
      });

      // Reload payment details to show updated status
      await loadPaymentDetails();
      
      if (onReverse) {
        onReverse(paymentId);
      }
    } catch (error: any) {
      toast({
        title: 'Reverse Failed',
        description: error.message || 'Failed to reverse payment',
        status: 'error',
        duration: 5000,
        isClosable: true
      });
    } finally {
      setIsReversing(false);
      onReverseModalClose();
    }
  };

  const refreshBalances = async () => {
    try {
      await paymentService.refreshAccountBalances();
      const balances = await paymentService.getAccountBalanceUpdates(numericPaymentId);
      setAccountBalances(balances);
      
      toast({
        title: 'Balances Refreshed',
        description: 'Account balances have been updated',
        status: 'success',
        duration: 3000,
        isClosable: true
      });
    } catch (error: any) {
      toast({
        title: 'Refresh Failed',
        description: error.message || 'Failed to refresh balances',
        status: 'error',
        duration: 5000,
        isClosable: true
      });
    }
  };

  if (isLoading) {
    return (
      <Box p={6}>
        <Card bg={bgColor}>
          <CardBody>
            <VStack spacing={4} py={8}>
              <Icon as={FiRefreshCw} boxSize={8} color="gray.400" />
              <Text color="gray.500">Loading payment details...</Text>
            </VStack>
          </CardBody>
        </Card>
      </Box>
    );
  }

  if (!paymentInfo) {
    return (
      <Box p={6}>
        <Alert status="error">
          <AlertIcon />
          Payment not found or failed to load
        </Alert>
      </Box>
    );
  }

  const { payment, journal_result: journalResult, contact, cash_bank: cashBank } = paymentInfo;
  const paymentType = payment.payment_type ?? 'SEND';
  const paymentCode = payment.code || `#${payment.id}`;
  const paymentMethod = payment.method || '-';
  const paymentNotes = payment.notes?.trim();
  const paymentReference = payment.reference?.trim();
  const statusLabel = paymentService.getStatusLabel(payment.status);
  const approverName = paymentService.getActorDisplayName(payment.approver);
  const rejectorName = paymentService.getActorDisplayName(payment.rejector);

  const StatusBadge = ({ status }: { status: string }) => {
    const colorScheme = paymentService.getStatusColorScheme(status);
    
    return (
      <Badge colorScheme={colorScheme} variant="solid">
        {paymentService.getStatusLabel(status)}
      </Badge>
    );
  };

  const PaymentTypeIcon = ({ type }: { type: string }) => {
    return type === 'RECEIVE' ? 
      <Icon as={FiArrowDown} color="green.500" /> : 
      <Icon as={FiArrowRight} color="blue.500" />;
  };

  return (
    <Box maxW="6xl" mx="auto" p={6}>
      <VStack spacing={6} align="stretch">
        {/* Header */}
        <Card bg={bgColor}>
          <CardHeader bg={headerBg}>
            <Flex align="center">
              <HStack spacing={3}>
                <PaymentTypeIcon type={paymentType} />
                <VStack align="start" spacing={0}>
                  <Heading size="md" color={textPrimary}>
                    Payment {paymentCode}
                  </Heading>
                  <Text fontSize="sm" color={textSecondary}>
                    Created {new Date(payment.created_at).toLocaleString()}
                  </Text>
                </VStack>
              </HStack>
              <Spacer />
              <HStack>
                <StatusBadge status={payment.status} />
                {onEdit && payment.status === 'DRAFT' && (
                  <Button size="sm" variant="outline" onClick={() => onEdit(paymentId)}>
                    Edit
                  </Button>
                )}
                {payment.status === 'COMPLETED' && (
                  <Button
                    size="sm"
                    colorScheme="red"
                    variant="outline"
                    leftIcon={<Icon as={FiRotateCcw} />}
                    onClick={onReverseModalOpen}
                  >
                    Reverse
                  </Button>
                )}
              </HStack>
            </Flex>
          </CardHeader>
        </Card>

        <Tabs colorScheme="blue">
          <TabList>
            <Tab>Payment Details</Tab>
            <Tab>Journal Entry</Tab>
            {accountBalances.length > 0 && <Tab>Account Balances</Tab>}
          </TabList>

          <TabPanels>
            {/* Payment Details Tab */}
            <TabPanel px={0}>
              <Card bg={bgColor}>
                <CardBody>
                  <VStack spacing={6} align="stretch">
                    {/* Payment Summary */}
                    <HStack spacing={8} align="start">
                      <Stat>
                        <StatLabel>Amount</StatLabel>
                        <StatNumber fontSize="2xl">
                          {formatCurrency(payment.amount)}
                        </StatNumber>
                        <StatHelpText>{payment.currency}</StatHelpText>
                      </Stat>
                      <Stat>
                        <StatLabel>Payment Method</StatLabel>
                        <StatNumber fontSize="lg">
                          {paymentMethod.replace(/_/g, ' ')}
                        </StatNumber>
                      </Stat>
                      <Stat>
                        <StatLabel>Type</StatLabel>
                        <StatNumber fontSize="lg" color={paymentType === 'RECEIVE' ? 'green.500' : 'blue.500'}>
                          {paymentType}
                        </StatNumber>
                      </Stat>
                    </HStack>

                    <Divider />

                    {/* Payment Details Grid */}
                    <VStack spacing={4} align="stretch">
                      {paymentNotes ? (
                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Notes</Text>
                          <Text color={textSecondary}>{paymentNotes}</Text>
                        </Box>
                      ) : null}

                      <HStack spacing={8} align="start">
                        {paymentReference ? (
                          <Box>
                            <Text fontWeight="medium" color={textPrimary}>Reference</Text>
                            <Text color={textSecondary}>{paymentReference}</Text>
                          </Box>
                        ) : null}

                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Contact</Text>
                          <Text color={textSecondary}>
                            {contact?.name || payment.contact?.name || '-'}
                          </Text>
                        </Box>

                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Contact ID</Text>
                          <Text color={textSecondary}>{payment.contact_id}</Text>
                        </Box>

                        {cashBank ? (
                          <Box>
                            <Text fontWeight="medium" color={textPrimary}>Cash / Bank</Text>
                            <Text color={textSecondary}>
                              {cashBank.account_code} - {cashBank.name}
                            </Text>
                          </Box>
                        ) : null}
                      </HStack>

                      <HStack spacing={8}>
                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Created</Text>
                          <Text color={textSecondary}>
                            {new Date(payment.created_at).toLocaleString()}
                          </Text>
                        </Box>
                        
                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Updated</Text>
                          <Text color={textSecondary}>
                            {new Date(payment.updated_at).toLocaleString()}
                          </Text>
                        </Box>

                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Status</Text>
                          <Text color={textSecondary}>{statusLabel}</Text>
                        </Box>
                      </HStack>

                      {payment.approved_at && (
                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Approval</Text>
                          <Text color={textSecondary}>
                            Approved by {approverName} pada {new Date(payment.approved_at).toLocaleString('id-ID')}
                          </Text>
                        </Box>
                      )}

                      {payment.rejected_at && (
                        <Box>
                          <Text fontWeight="medium" color={textPrimary}>Rejection</Text>
                          <Text color={textSecondary}>
                            Rejected by {rejectorName} pada {new Date(payment.rejected_at).toLocaleString('id-ID')}
                          </Text>
                          {payment.rejection_reason && (
                            <Text color="red.500" fontSize="sm" mt={1}>
                              {payment.rejection_reason}
                            </Text>
                          )}
                        </Box>
                      )}
                    </VStack>
                  </VStack>
                </CardBody>
              </Card>
            </TabPanel>

            {/* Journal Entry Tab */}
            <TabPanel px={0}>
              {journalResult ? (
                <VStack spacing={4} align="stretch">
                  {/* Journal Info Summary */}
                  <Card bg={bgColor} borderWidth="1px" borderColor={borderColor}>
                    <CardBody>
                      <HStack spacing={4} justify="space-between">
                        <VStack align="start" spacing={1}>
                          <Text fontWeight="medium" color={textPrimary}>Journal Entry</Text>
                          <Tag size="md" colorScheme="blue">
                            <TagLeftIcon as={FiFileText} />
                            <TagLabel>{journalResult.journal_entry.entry_number}</TagLabel>
                          </Tag>
                        </VStack>
                        
                        <VStack align="end" spacing={1}>
                          <Text fontWeight="medium" color={textPrimary}>Status</Text>
                          <StatusBadge status={journalResult.journal_entry.status} />
                        </VStack>

                        <Button
                          size="sm"
                          variant="outline"
                          leftIcon={<Icon as={FiExternalLink} />}
                        >
                          View Full Journal
                        </Button>
                      </HStack>
                    </CardBody>
                  </Card>

                  {/* Journal Entry Details */}
                  <JournalEntryPreview
                    journalResult={{
                      ...journalResult,
                      journal_entry: {
                        ...journalResult.journal_entry,
                        lines: journalResult.journal_entry.lines || [],
                        created_at: journalResult.journal_entry.created_at || payment.created_at,
                        updated_at: journalResult.journal_entry.updated_at || payment.updated_at,
                      } as JournalEntry,
                      account_updates: accountBalances.length > 0 ? accountBalances : journalResult.account_updates,
                    }}
                    title="Journal Entry Details"
                    isPreview={false}
                    showAccountUpdates={false}
                  />
                </VStack>
              ) : (
                <Card bg={bgColor}>
                  <CardBody>
                    <VStack spacing={4} py={8}>
                      <Icon as={FiFileText} boxSize={8} color="gray.400" />
                      <Text color="gray.500">No journal entry associated with this payment</Text>
                    </VStack>
                  </CardBody>
                </Card>
              )}
            </TabPanel>

            {/* Account Balances Tab */}
            {accountBalances.length > 0 && (
              <TabPanel px={0}>
                <Card bg={bgColor}>
                  <CardHeader>
                    <Flex align="center">
                      <HStack>
                        <Icon as={FiDollarSign} color="blue.500" />
                        <Heading size="sm" color={textPrimary}>Account Balance Changes</Heading>
                      </HStack>
                      <Spacer />
                      <Button
                        size="sm"
                        variant="outline"
                        leftIcon={<Icon as={FiRefreshCw} />}
                        onClick={refreshBalances}
                      >
                        Refresh Balances
                      </Button>
                    </Flex>
                  </CardHeader>
                  <CardBody>
                    <VStack spacing={3} align="stretch">
                      {accountBalances.map((update, index) => (
                        <Box key={update.account_id || index} p={4} borderWidth="1px" borderColor={borderColor} borderRadius="md">
                          <HStack justify="space-between" mb={2}>
                            <VStack align="start" spacing={0}>
                              <Text fontWeight="medium" color={textPrimary}>
                                {update.account_code} - {update.account_name}
                              </Text>
                              <Text fontSize="sm" color={textSecondary}>
                                Account ID: {update.account_id}
                              </Text>
                            </VStack>
                            <Badge colorScheme={update.change_type === 'INCREASE' ? 'green' : 'red'}>
                              {update.change_type === 'INCREASE' ? '+' : '-'} {formatCurrency(Math.abs(update.change))}
                            </Badge>
                          </HStack>
                          <HStack spacing={4}>
                            <Text fontSize="sm" color={textSecondary}>
                              Previous: {formatCurrency(update.old_balance)}
                            </Text>
                            <Icon as={FiArrowRight} color="gray.400" boxSize={3} />
                            <Text fontSize="sm" color={textSecondary}>
                              New: {formatCurrency(update.new_balance)}
                            </Text>
                          </HStack>
                        </Box>
                      ))}
                    </VStack>
                  </CardBody>
                </Card>
              </TabPanel>
            )}
          </TabPanels>
        </Tabs>
      </VStack>

      {/* Reverse Payment Modal */}
      <Modal isOpen={isReverseModalOpen} onClose={onReverseModalClose}>
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Reverse Payment</ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            <VStack spacing={4} align="start">
              <Alert status="warning">
                <AlertIcon />
                <Text fontSize="sm">
                  This will reverse the payment and create reversing journal entries. This action cannot be undone.
                </Text>
              </Alert>
              <Text>
                Are you sure you want to reverse payment <strong>{paymentCode}</strong> for{' '}
                <strong>{formatCurrency(payment.amount)}</strong>?
              </Text>
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button variant="ghost" mr={3} onClick={onReverseModalClose}>
              Cancel
            </Button>
            <Button
              colorScheme="red"
              onClick={handleReversePayment}
              isLoading={isReversing}
              loadingText="Reversing..."
            >
              Reverse Payment
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>

      {/* Reverse Result Modal */}
      {reverseResult && (
        <Modal isOpen={!!reverseResult} onClose={() => setReverseResult(null)} size="lg">
          <ModalOverlay />
          <ModalContent>
            <ModalHeader>Payment Reversal Complete</ModalHeader>
            <ModalCloseButton />
            <ModalBody>
              <JournalEntryPreview
                journalResult={reverseResult}
                title="Reversal Journal Entry"
                isPreview={false}
                showAccountUpdates={true}
              />
            </ModalBody>
            <ModalFooter>
              <Button onClick={() => setReverseResult(null)}>
                Close
              </Button>
            </ModalFooter>
          </ModalContent>
        </Modal>
      )}
    </Box>
  );
};

export default PaymentDetailsView;
