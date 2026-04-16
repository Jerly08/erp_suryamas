'use client';

import React from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Button,
  Box,
  Flex,
  Text,
  Badge,
  Divider,
  HStack,
  VStack,
  Table,
  Tbody,
  Tr,
  Td,
  TableContainer,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiFilePlus } from 'react-icons/fi';
import { Payment, stripInternalPaymentReference } from '@/services/paymentService';
import paymentService from '@/services/paymentService';

interface PaymentDetailModalProps {
  payment: Payment | null;
  isOpen: boolean;
  onClose: () => void;
}

const PaymentDetailModal: React.FC<PaymentDetailModalProps> = ({
  payment,
  isOpen,
  onClose,
}) => {
  // Theme colors for dark mode support
  const modalBg = useColorModeValue('white', 'gray.800');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const headingColor = useColorModeValue('gray.800', 'gray.100');
  const textSecondary = useColorModeValue('gray.500', 'gray.400');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const tableBg = useColorModeValue('white', 'gray.700');
  const amountColor = useColorModeValue('green.600', 'green.400');
  
  if (!payment) return null;

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
    }).format(amount);
  };

  const formatDateTime = (dateString: string) => {
    return new Date(dateString).toLocaleString('id-ID', {
      day: '2-digit',
      month: '2-digit',
      year: 'numeric',
      hour: '2-digit',
      minute: '2-digit',
    });
  };

  const statusLabel = paymentService.getStatusLabel(payment.status);
  const approvedBy = paymentService.getActorDisplayName(payment.approver);
  const rejectedBy = paymentService.getActorDisplayName(payment.rejector);

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="xl">
      <ModalOverlay />
      <ModalContent bg={modalBg}>
        <ModalHeader bg={headerBg} borderBottomWidth={1} borderColor={borderColor}>
          <Flex justify="space-between" align="center">
            <Text color={headingColor}>Payment Details</Text>
            <Badge
              colorScheme={paymentService.getStatusColorScheme(payment.status)}
              variant="subtle"
              fontSize="sm"
            >
              {statusLabel}
            </Badge>
          </Flex>
        </ModalHeader>
        <ModalCloseButton />
        
        <ModalBody>
          <VStack spacing={6} align="stretch">
            {/* Basic Information */}
            <Box>
              <Text fontWeight="bold" fontSize="lg" mb={3} color={headingColor}>
                Basic Information
              </Text>
              <TableContainer>
                <Table size="sm" variant="simple" bg={tableBg}>
                  <Tbody>
                    <Tr>
                      <Td fontWeight="medium" w="30%">Payment Code:</Td>
                      <Td>{payment.code}</Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">Contact:</Td>
                      <Td>
                        <Flex direction="column">
                          <Text>
                            {/* For PPN tax payments, show "Negara" instead of contact name */}
                            {(payment.payment_type === 'TAX_PPN' || 
                              payment.payment_type === 'TAX_PPN_INPUT' || 
                              payment.payment_type === 'TAX_PPN_OUTPUT' ||
                              payment.code?.startsWith('SETOR-PPN')) 
                              ? 'Negara' 
                              : (payment.contact?.name || 'Unknown Contact')
                            }
                          </Text>
                          <Text fontSize="sm" color={textSecondary}>
                            {(payment.payment_type === 'TAX_PPN' || 
                              payment.payment_type === 'TAX_PPN_INPUT' || 
                              payment.payment_type === 'TAX_PPN_OUTPUT' ||
                              payment.code?.startsWith('SETOR-PPN'))
                              ? 'Pemerintah / Tax Authority'
                              : (payment.contact?.type || 'N/A')
                            }
                          </Text>
                        </Flex>
                      </Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">Amount:</Td>
                      <Td>
                        <Text fontWeight="bold" fontSize="lg" color={amountColor}>
                          {formatCurrency(payment.amount)}
                        </Text>
                      </Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">Payment Date:</Td>
                      <Td>{formatDateTime(payment.date)}</Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">Method:</Td>
                      <Td>
                        <Badge variant="outline">
                          {paymentService.getMethodDisplayName(payment.method)}
                        </Badge>
                      </Td>
                    </Tr>
                  </Tbody>
                </Table>
              </TableContainer>
            </Box>

            <Divider />

            {/* Additional Details */}
            <Box>
              <Text fontWeight="bold" fontSize="lg" mb={3} color={headingColor}>
                Additional Information
              </Text>
              <TableContainer>
                <Table size="sm" variant="simple" bg={tableBg}>
                  <Tbody>
                    <Tr>
                      <Td fontWeight="medium" w="30%">Reference:</Td>
                      <Td>{stripInternalPaymentReference(payment.reference) || '-'}</Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">Notes:</Td>
                      <Td>
                        {payment.notes ? (
                          <Text fontSize="sm" whiteSpace="pre-wrap">
                            {payment.notes}
                          </Text>
                        ) : (
                          <Text color={textSecondary} fontSize="sm">No notes</Text>
                        )}
                      </Td>
                    </Tr>
                  </Tbody>
                </Table>
              </TableContainer>
            </Box>

            <Divider />

            {/* System Information */}
            <Box>
              <Text fontWeight="bold" fontSize="lg" mb={3} color={headingColor}>
                System Information
              </Text>
              <TableContainer>
                <Table size="sm" variant="simple" bg={tableBg}>
                  <Tbody>
                    <Tr>
                      <Td fontWeight="medium" w="30%">Created:</Td>
                      <Td>{formatDateTime(payment.created_at)}</Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">Last Updated:</Td>
                      <Td>{formatDateTime(payment.updated_at)}</Td>
                    </Tr>
                    <Tr>
                      <Td fontWeight="medium">User ID:</Td>
                      <Td>{payment.user_id}</Td>
                    </Tr>
                    {payment.approved_at && (
                      <Tr>
                        <Td fontWeight="medium">Approved By:</Td>
                        <Td>
                          <Flex direction="column">
                            <Text>{approvedBy}</Text>
                            <Text fontSize="sm" color={textSecondary}>
                              {formatDateTime(payment.approved_at)}
                            </Text>
                          </Flex>
                        </Td>
                      </Tr>
                    )}
                    {payment.rejected_at && (
                      <Tr>
                        <Td fontWeight="medium">Rejected By:</Td>
                        <Td>
                          <Flex direction="column">
                            <Text>{rejectedBy}</Text>
                            <Text fontSize="sm" color={textSecondary}>
                              {formatDateTime(payment.rejected_at)}
                            </Text>
                            {payment.rejection_reason && (
                              <Text fontSize="sm" color="red.500">
                                {payment.rejection_reason}
                              </Text>
                            )}
                          </Flex>
                        </Td>
                      </Tr>
                    )}
                  </Tbody>
                </Table>
              </TableContainer>
            </Box>
          </VStack>
        </ModalBody>

        <ModalFooter>
          <HStack spacing={3}>
            <Button
              variant="outline"
              leftIcon={<FiFilePlus />}
onClick={async () => {
              try {
                await paymentService.downloadPaymentDetailPDF(payment.id, payment.code);
              } catch (e) {
                console.error('Download payment PDF failed', e);
              }
            }}
              colorScheme="red"
            >
              Export to PDF
            </Button>
            <Button onClick={onClose}>Close</Button>
          </HStack>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default PaymentDetailModal;
