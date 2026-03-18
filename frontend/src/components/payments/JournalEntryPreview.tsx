'use client';

import React from 'react';
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
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Alert,
  AlertIcon,
  Collapse,
  Icon,
  useColorModeValue,
  Flex,
  Spacer,
  Button,
} from '@chakra-ui/react';
import { FiChevronDown, FiChevronUp, FiFileText, FiDollarSign } from 'react-icons/fi';
import { PaymentJournalResult, JournalEntry, AccountBalanceUpdate } from '@/services/paymentService';

interface JournalEntryPreviewProps {
  journalResult: PaymentJournalResult | null;
  title?: string;
  isPreview?: boolean;
  showAccountUpdates?: boolean;
  isExpanded?: boolean;
  onToggleExpand?: () => void;
}

// Currency formatter
const formatCurrency = (amount: number) => {
  return new Intl.NumberFormat('id-ID', {
    style: 'currency',
    currency: 'IDR',
    minimumFractionDigits: 0,
    maximumFractionDigits: 0
  }).format(amount);
};

const JournalEntryPreview: React.FC<JournalEntryPreviewProps> = ({
  journalResult,
  title = 'Journal Entry',
  isPreview = false,
  showAccountUpdates = true,
  isExpanded = false,
  onToggleExpand
}) => {
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const textSecondary = useColorModeValue('gray.600', 'gray.400');
  const textPrimary = useColorModeValue('gray.700', 'gray.200');

  if (!journalResult) {
    return null;
  }

  const { journal_entry: journalEntry, account_updates: accountUpdates, success, message } = journalResult;

  const StatusBadge = ({ status }: { status: string }) => {
    const colorScheme = status === 'POSTED' ? 'green' : 
                       status === 'PREVIEW' ? 'blue' : 
                       status === 'DRAFT' ? 'yellow' : 'gray';
    
    return (
      <Badge colorScheme={colorScheme} variant="solid" size="sm">
        {status}
      </Badge>
    );
  };

  return (
    <Card bg={bgColor} borderWidth="1px" borderColor={borderColor} shadow="sm">
      <CardHeader bg={headerBg} py={3}>
        <Flex align="center">
          <HStack spacing={3}>
            <Icon as={FiFileText} color="blue.500" boxSize={5} />
            <Heading size="sm" color={textPrimary}>{title}</Heading>
            {isPreview && (
              <Badge colorScheme="blue" variant="outline">Preview</Badge>
            )}
          </HStack>
          <Spacer />
          {onToggleExpand && (
            <Button
              size="xs"
              variant="ghost"
              onClick={onToggleExpand}
              rightIcon={<Icon as={isExpanded ? FiChevronUp : FiChevronDown} />}
            >
              {isExpanded ? 'Collapse' : 'Expand'}
            </Button>
          )}
        </Flex>
      </CardHeader>

      {!onToggleExpand || isExpanded ? (
        <CardBody>
          <VStack spacing={4} align="stretch">
            {/* Success/Error Message */}
            {message && (
              <Alert status={success ? 'success' : 'error'} size="sm">
                <AlertIcon />
                <Text fontSize="sm">{message}</Text>
              </Alert>
            )}

            {/* Journal Entry Header Info */}
            {journalEntry && (
              <Box>
                <HStack justify="space-between" mb={3}>
                  <VStack align="start" spacing={1}>
                    <Text fontSize="sm" color={textSecondary}>Entry Number</Text>
                    <Text fontWeight="medium" color={textPrimary}>
                      {journalEntry.entry_number}
                    </Text>
                  </VStack>
                  <VStack align="end" spacing={1}>
                    <Text fontSize="sm" color={textSecondary}>Status</Text>
                    <StatusBadge status={journalEntry.status} />
                  </VStack>
                </HStack>

                {/* Balance Summary */}
                <HStack justify="space-between" mb={4} p={3} bg={headerBg} borderRadius="md">
                  <VStack align="start" spacing={1}>
                    <Text fontSize="xs" color={textSecondary}>Total Debit</Text>
                    <Text fontWeight="medium" color="green.500">
                      {formatCurrency(journalEntry.total_debit)}
                    </Text>
                  </VStack>
                  <VStack align="center" spacing={1}>
                    <Text fontSize="xs" color={textSecondary}>Balanced</Text>
                    <Badge colorScheme={journalEntry.is_balanced ? 'green' : 'red'}>
                      {journalEntry.is_balanced ? '✓' : '✗'}
                    </Badge>
                  </VStack>
                  <VStack align="end" spacing={1}>
                    <Text fontSize="xs" color={textSecondary}>Total Credit</Text>
                    <Text fontWeight="medium" color="red.500">
                      {formatCurrency(journalEntry.total_credit)}
                    </Text>
                  </VStack>
                </HStack>
              </Box>
            )}

            {/* Journal Lines */}
            {journalEntry?.lines && journalEntry.lines.length > 0 && (
              <Box>
                <Text fontWeight="medium" mb={3} color={textPrimary}>Journal Lines</Text>
                <Table size="sm" variant="simple">
                  <Thead>
                    <Tr>
                      <Th>Description</Th>
                      <Th textAlign="right">Debit</Th>
                      <Th textAlign="right">Credit</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {journalEntry.lines.map((line, index) => (
                      <Tr key={line.id || index}>
                        <Td>
                          <VStack align="start" spacing={0}>
                            <Text fontSize="sm" color={textPrimary}>
                              {line.description}
                            </Text>
                            <Text fontSize="xs" color={textSecondary}>
                              Account ID: {line.account_id}
                            </Text>
                          </VStack>
                        </Td>
                        <Td textAlign="right">
                          {line.debit_amount > 0 && (
                            <Text fontWeight="medium" color="green.600">
                              {formatCurrency(line.debit_amount)}
                            </Text>
                          )}
                        </Td>
                        <Td textAlign="right">
                          {line.credit_amount > 0 && (
                            <Text fontWeight="medium" color="red.600">
                              {formatCurrency(line.credit_amount)}
                            </Text>
                          )}
                        </Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              </Box>
            )}

            {/* Account Balance Updates */}
            {showAccountUpdates && accountUpdates && accountUpdates.length > 0 && (
              <>
                <Divider />
                <Box>
                  <HStack mb={3}>
                    <Icon as={FiDollarSign} color="blue.500" />
                    <Text fontWeight="medium" color={textPrimary}>Account Balance Changes</Text>
                  </HStack>
                  <VStack spacing={2} align="stretch">
                    {accountUpdates.map((update, index) => (
                      <Box key={update.account_id || index} p={3} borderWidth="1px" borderColor={borderColor} borderRadius="md">
                        <HStack justify="space-between" mb={1}>
                          <VStack align="start" spacing={0}>
                            <Text fontSize="sm" fontWeight="medium" color={textPrimary}>
                              {update.account_code} - {update.account_name}
                            </Text>
                            <Text fontSize="xs" color={textSecondary}>
                              {formatCurrency(update.old_balance)} → {formatCurrency(update.new_balance)}
                            </Text>
                          </VStack>
                          <VStack align="end" spacing={0}>
                            <Badge colorScheme={update.change_type === 'INCREASE' ? 'green' : 'red'}>
                              {update.change_type === 'INCREASE' ? '+' : '-'} {formatCurrency(Math.abs(update.change))}
                            </Badge>
                          </VStack>
                        </HStack>
                      </Box>
                    ))}
                  </VStack>
                </Box>
              </>
            )}

            {/* Preview Warning */}
            {isPreview && (
              <Alert status="info" size="sm">
                <AlertIcon />
                <Text fontSize="sm">
                  This is a preview. The actual journal entry will be created when you submit the payment.
                </Text>
              </Alert>
            )}
          </VStack>
        </CardBody>
      ) : null}
    </Card>
  );
};

export default JournalEntryPreview;
