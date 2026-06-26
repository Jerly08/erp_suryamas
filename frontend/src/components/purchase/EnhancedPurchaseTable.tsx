'use client';

import React from 'react';
import {
  Box,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Badge,
  Text,
  Flex,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  MenuDivider,
  IconButton,
  HStack,
  Spinner,
  useColorModeValue,
  TableContainer,
  Card,
  CardHeader,
  CardBody,
  Heading,
} from '@chakra-ui/react';
import {
  FiMoreVertical,
  FiEye,
  FiEdit,
  FiCheck,
  FiX,
  FiAlertCircle,
  FiClock,
  FiTrash2,
  FiDollarSign,
  FiCopy
} from 'react-icons/fi';
import { Purchase } from '@/services/purchaseService';
import { useTranslation } from '@/hooks/useTranslation';
import { getPurchasePPhStatus } from '@/utils/taxPaymentStatus';

interface PurchaseTableProps {
  purchases: Purchase[];
  loading: boolean;
  onViewDetails: (purchase: Purchase) => void;
  onEdit?: (purchase: Purchase) => void;
  onSubmitForApproval?: (purchaseId: number) => void;
  onDelete?: (purchaseId: number) => void;
  onRecordPayment?: (purchase: Purchase) => void;
  onDuplicate?: (purchase: Purchase) => void;
  renderActions?: (purchase: Purchase) => React.ReactNode;
  title?: string;
  formatCurrency: (amount: number) => string;
  formatDate: (date: string) => string;
  canEdit?: boolean;
  canDelete?: boolean;
  userRole?: string;
}

const EnhancedPurchaseTable: React.FC<PurchaseTableProps> = ({
  purchases,
  loading,
  onViewDetails,
  onEdit,
  onSubmitForApproval,
  onDelete,
  onRecordPayment,
  onDuplicate,
  renderActions,
  title = 'Purchase Transactions',
  formatCurrency,
  formatDate,
  canEdit = false,
  canDelete = false,
  userRole,
}) => {
  const { t } = useTranslation();

  // Theme colors with improved compatibility - using Chakra theme tokens with CSS variable fallbacks
  const headingColor = useColorModeValue('gray.800', 'gray.100');
  const tableBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const textColor = useColorModeValue('gray.600', 'gray.400');
  const primaryTextColor = useColorModeValue('gray.800', 'gray.100');
  const hoverBg = useColorModeValue('gray.50', 'gray.700');
  const theadBg = useColorModeValue('gray.50', 'gray.700');
  const spinnerColor = useColorModeValue('blue.500', 'blue.400');

  // Status color mapping for purchases
  const getStatusColor = (status: string) => {
    switch (status?.toLowerCase()) {
      case 'approved':
      case 'completed':
        return 'green';
      case 'paid':
        return 'teal'; // Special color for PAID status to distinguish from other green statuses
      case 'draft':
      case 'pending_approval':
        return 'yellow';
      case 'pending':
        return 'blue';
      case 'pending_accounting':
        return 'orange';
      case 'pending_final_approval':
        return 'purple';
      case 'cancelled':
      case 'rejected':
        return 'red';
      default:
        return 'gray';
    }
  };

  // Approval status color mapping
  const getApprovalStatusColor = (approvalStatus: string) => {
    switch ((approvalStatus || '').toLowerCase()) {
      case 'approved':
        return 'green';
      case 'pending':
        return 'yellow';
      case 'rejected':
        return 'red';
      case 'not_required':
      case 'not_started':
        return 'gray';
      default:
        return 'gray';
    }
  };

  const translateOrFallback = (key: string, fallback: string) => {
    const translated = t(key);
    return translated === key ? fallback : translated;
  };

  const getStatusLabel = (status: string) => {
    const statusKey = status.toLowerCase();
    const translationKey = `purchases.statuses.${statusKey}`;
    const fallback = status.replace(/_/g, ' ').toUpperCase();
    return translateOrFallback(translationKey, fallback);
  };

  const getApprovalStatusLabel = (approvalStatus: string) => {
    const statusKey = (approvalStatus || '').toLowerCase();
    const translationKey = `purchases.approvalStatuses.${statusKey}`;
    const fallback = (approvalStatus || '').replace(/_/g, ' ').toUpperCase() || '-';
    return translateOrFallback(translationKey, fallback);
  };

  const getPurchaseRequestNumber = (purchase: Purchase) =>
    purchase.purchase_request?.pr_number ||
    purchase.purchase_request?.request_number ||
    (purchase.purchase_request_id ? `#${purchase.purchase_request_id}` : '');

  const getPurchaseRequestTitle = (purchase: Purchase) =>
    purchase.purchase_request?.title?.trim() || '';

  const getPurchaseRequestDepartment = (purchase: Purchase) =>
    purchase.purchase_request?.department?.trim() || '';

  return (
    <Card boxShadow="sm" borderRadius="lg" borderWidth="1px" borderColor={borderColor}>
      <CardHeader>
        <Flex justify="space-between" align="center">
          <Heading size="md" color={headingColor}>
            {title} ({purchases?.length || 0})
          </Heading>
        </Flex>
      </CardHeader>
      <CardBody p={0}>
        {loading ? (
          <Flex justify="center" align="center" py={10}>
            <Spinner size="lg" color={spinnerColor} />
            <Text ml={4} color={textColor}>{t('purchases.loadingTransactions')}</Text>
          </Flex>
        ) : purchases.length === 0 ? (
          <Box p={8} textAlign="center">
            <Text color={textColor}>{t('purchases.noTransactionsFound')}</Text>
          </Box>
        ) : (
          <Box overflowX="auto">
            <Table variant="simple" size="md" className="table">
              <Thead bg={theadBg}>
                <Tr>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.purchaseNumber', 'PURCHASE #')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.vendor', 'VENDOR')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">PR / Divisi / Project</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.date', 'DATE')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.total', 'TOTAL')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.paidAmount', 'PAID AMOUNT')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.outstanding', 'OUTSTANDING')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.status', 'STATUS')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">STATUS PPH</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{translateOrFallback('purchases.table.approvalStatus', 'APPROVAL STATUS')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold" textAlign="center">{translateOrFallback('purchases.table.actions', 'ACTIONS')}</Th>
                </Tr>
              </Thead>
              <Tbody>
                {purchases.map((purchase, index) => {
                  const pphStatus = getPurchasePPhStatus(purchase);

                  return (
                    <Tr
                      key={purchase.id}
                      _hover={{ bg: hoverBg }}
                      transition="all 0.2s ease"
                      borderBottom={index === purchases.length - 1 ? 'none' : '1px solid'}
                      borderColor={borderColor}
                    >
                    <Td borderColor={borderColor} py={3}>
                      <Text fontWeight="medium" color="blue.600">
                        {purchase.code}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontWeight="medium" color={primaryTextColor} fontSize="sm">
                        {purchase.vendor?.name || 'N/A'}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3} minW="190px">
                      {purchase.purchase_request_id || purchase.project_id ? (
                        <Flex direction="column" align="flex-start" gap={1}>
                          {purchase.purchase_request_id && (
                            <>
                              <Badge colorScheme="blue" variant="subtle" fontSize="2xs">
                                PR: {getPurchaseRequestNumber(purchase)}
                              </Badge>
                              {getPurchaseRequestTitle(purchase) && (
                                <Text
                                  fontSize="xs"
                                  color={primaryTextColor}
                                  fontWeight="semibold"
                                  noOfLines={2}
                                >
                                  Judul: {getPurchaseRequestTitle(purchase)}
                                </Text>
                              )}
                              {getPurchaseRequestDepartment(purchase) && (
                                <Text fontSize="xs" color={textColor} noOfLines={1}>
                                  Divisi: {getPurchaseRequestDepartment(purchase)}
                                </Text>
                              )}
                            </>
                          )}
                          {purchase.project_id && (
                            <Badge colorScheme="purple" variant="subtle" fontSize="2xs">
                              PROJ: {purchase.project?.code || `#${purchase.project_id}`}
                            </Badge>
                          )}
                        </Flex>
                      ) : (
                        <Text fontSize="xs" color="gray.400">-</Text>
                      )}
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontSize="sm" color={textColor}>
                        {formatDate(purchase.date)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontWeight="medium" fontSize="sm" color={primaryTextColor}>
                        {formatCurrency(purchase.total_amount)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontSize="sm" color={purchase.paid_amount > 0 ? "green.600" : textColor}>
                        {formatCurrency(purchase.paid_amount || 0)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text
                        fontSize="sm"
                        color={purchase.outstanding_amount > 0 ? "orange.600" : "green.600"}
                        fontWeight={purchase.outstanding_amount > 0 ? "semibold" : "normal"}
                      >
                        {formatCurrency(purchase.outstanding_amount || 0)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Badge
                        colorScheme={getStatusColor(purchase.status)}
                        variant="subtle"
                        px={2}
                        py={1}
                        borderRadius="md"
                        fontSize="xs"
                      >
                        {getStatusLabel(purchase.status)}
                      </Badge>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Flex direction="column" align="flex-start" gap={1}>
                        <Badge
                          colorScheme={pphStatus.colorScheme}
                          variant="subtle"
                          px={2}
                          py={1}
                          borderRadius="md"
                          fontSize="xs"
                        >
                          {pphStatus.label}
                        </Badge>
                        {pphStatus.totalAmount > 0 && (
                          <Text fontSize="xs" color={textColor}>
                            PPh: {formatCurrency(pphStatus.totalAmount)}
                          </Text>
                        )}
                        {pphStatus.outstandingAmount > 0 && (
                          <Text fontSize="xs" color="orange.500">
                            Sisa: {formatCurrency(pphStatus.outstandingAmount)}
                          </Text>
                        )}
                      </Flex>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Badge
                        colorScheme={getApprovalStatusColor(purchase.approval_status)}
                        variant="subtle"
                        px={2}
                        py={1}
                        borderRadius="md"
                        fontSize="xs"
                      >
                        {getApprovalStatusLabel(purchase.approval_status)}
                      </Badge>
                    </Td>
                    <Td borderColor={borderColor} py={3} textAlign="center">
                      {renderActions ? (
                        renderActions(purchase)
                      ) : (
                        <Menu>
                          <MenuButton
                            as={IconButton}
                            icon={<FiMoreVertical />}
                            variant="ghost"
                            size="sm"
                            aria-label="Options"
                          />
                          <MenuList>
                            <MenuItem
                              icon={<FiEye />}
                              onClick={() => onViewDetails(purchase)}
                            >
                              {t('purchases.viewDetails')}
                            </MenuItem>

                            {onDuplicate && (
                              <MenuItem
                                icon={<FiCopy />}
                                onClick={() => onDuplicate(purchase)}
                              >
                                {t('common.duplicate')}
                              </MenuItem>
                            )}

                            {purchase.status === 'DRAFT' && canEdit && onEdit && (
                              <MenuItem icon={<FiEdit />} onClick={() => onEdit(purchase)}>
                                {t('purchases.edit')}
                              </MenuItem>
                            )}
                            {purchase.status === 'DRAFT' && onSubmitForApproval && (purchase.approval_status === 'NOT_STARTED' || !purchase.approval_status) && (
                              <MenuItem
                                icon={<FiAlertCircle />}
                                onClick={() => onSubmitForApproval(purchase.id)}
                              >
                                {t('purchases.submitForApproval')}
                              </MenuItem>
                            )}
                            {/* Record Payment - Show for APPROVED, COMPLETED, or PAID credit purchases with outstanding amount */}
                            {(purchase.status === 'APPROVED' || purchase.status === 'COMPLETED' || purchase.status === 'PAID') &&
                              purchase.payment_method === 'CREDIT' &&
                              (purchase.outstanding_amount || 0) > 0 &&
                              onRecordPayment && (
                                <>
                                  <MenuDivider />
                                  <MenuItem
                                    icon={<FiDollarSign />}
                                    onClick={() => onRecordPayment(purchase)}
                                    color="green.600"
                                  >
                                    {t('purchases.recordPayment')}
                                  </MenuItem>
                                </>
                              )}
                            {canDelete && onDelete && (
                              <>
                                <MenuDivider />
                                <MenuItem
                                  icon={<FiTrash2 />}
                                  color="red.500"
                                  onClick={() => onDelete(purchase.id)}
                                >
                                  {t('purchases.delete')}
                                </MenuItem>
                              </>
                            )}
                          </MenuList>
                        </Menu>
                      )}
                    </Td>
                  </Tr>
                  );
                })}
              </Tbody>
            </Table>
          </Box>
        )}
      </CardBody>
    </Card>
  );
};

export default EnhancedPurchaseTable;
