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
  FiDollarSign,
  FiDownload,
  FiTrash2,
  FiXCircle,
  FiPrinter,
  FiCopy
} from 'react-icons/fi';
import { Sale } from '@/services/salesService';
import { useTranslation } from '@/hooks/useTranslation';

interface SalesTableProps {
  sales: Sale[];
  loading: boolean;
  onViewDetails: (sale: Sale) => void;
  onEdit?: (sale: Sale) => void;
  onConfirm?: (sale: Sale) => void;
  onCancel?: (sale: Sale) => void;
  onPayment?: (sale: Sale) => void;
  onDelete?: (sale: Sale) => void;
  onDownloadInvoice?: (sale: Sale) => void;
  onCreateReceipt?: (sale: Sale) => void;
  onDuplicate?: (sale: Sale) => void;
  title?: string;
  formatCurrency: (amount: number) => string;
  formatDate: (date: string) => string;
  canEdit?: boolean;
  canDelete?: boolean;
}

const EnhancedSalesTable: React.FC<SalesTableProps> = ({
  sales,
  loading,
  onViewDetails,
  onEdit,
  onConfirm,
  onCancel,
  onPayment,
  onDelete,
  onDownloadInvoice,
  onCreateReceipt,
  onDuplicate,
  title,
  formatCurrency,
  formatDate,
  canEdit = false,
  canDelete = false,
}) => {
  const { t } = useTranslation();
  const displayTitle = title || t('sales.title');

  // Get status label with translation
  const getStatusLabel = (status: string) => {
    const statusKey = status.toLowerCase();
    const translationKey = `sales.statuses.${statusKey}`;
    return t(translationKey);
  };

  // Debug logging for props
  console.log('EnhancedSalesTable props:', {
    salesCount: sales?.length,
    canEdit,
    canDelete,
    hasEditHandler: !!onEdit,
    hasConfirmHandler: !!onConfirm,
    hasDeleteHandler: !!onDelete,
    hasDownloadHandler: !!onDownloadInvoice
  });
  // Theme colors
  const headingColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const tableBg = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const primaryTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const hoverBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');
  const theadBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');

  // Get status color based on status
  const getStatusColor = (status: string) => {
    switch (status?.toLowerCase()) {
      case 'paid': return 'green';
      case 'invoiced': return 'blue';
      case 'confirmed': return 'purple';
      case 'overdue': return 'red';
      case 'draft': return 'gray';
      case 'cancelled': return 'red';
      default: return 'gray';
    }
  };

  return (
    <Card boxShadow="sm" borderRadius="lg" borderWidth="1px" borderColor={borderColor} position="relative" overflow="visible">
      <CardHeader>
        <Flex justify="space-between" align="center">
          <Heading size="md" color={headingColor}>
            {displayTitle} ({sales?.length || 0})
          </Heading>
        </Flex>
      </CardHeader>
      <CardBody p={0} position="relative" overflow="visible">
        {loading ? (
          <Flex justify="center" align="center" py={10}>
            <Spinner size="lg" color="var(--accent-color)" />
            <Text ml={4} color={textColor}>{t('common.table.loading')}</Text>
          </Flex>
        ) : sales.length === 0 ? (
          <Box p={8} textAlign="center">
            <Text color={textColor}>{t('common.table.emptyState')}</Text>
          </Box>
        ) : (
          <Box overflowX="auto" position="relative">
            <Table variant="simple" size="md" className="table">
              <Thead bg={theadBg}>
                <Tr>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{t('sales.table.code')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{t('sales.table.customer')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">Proyek</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{t('sales.table.date')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{t('sales.table.total')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{t('sales.table.outstanding')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold">{t('sales.table.status')}</Th>
                  <Th color={textColor} borderColor={borderColor} fontSize="xs" fontWeight="bold" textAlign="center">{t('common.actions').toUpperCase()}</Th>
                </Tr>
              </Thead>
              <Tbody>
                {sales.map((sale, index) => (
                  <Tr
                    key={sale.id}
                    _hover={{ bg: hoverBg }}
                    transition="all 0.2s ease"
                    borderBottom={index === sales.length - 1 ? 'none' : '1px solid'}
                    borderColor={borderColor}
                  >
                    <Td borderColor={borderColor} py={3}>
                      <Text fontWeight="medium" color="blue.600">
                        {sale.code}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontWeight="medium" color={primaryTextColor} fontSize="sm">
                        {sale.customer?.name || 'N/A'}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontSize="sm" color={textColor}>
                        {sale.project?.name || '-'}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontSize="sm" color={textColor}>
                        {formatDate(sale.date)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text fontWeight="medium" fontSize="sm" color={primaryTextColor}>
                        {formatCurrency(sale.total_amount)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Text
                        fontWeight="medium"
                        fontSize="sm"
                        color={sale.outstanding_amount > 0 ? 'orange.600' : 'green.600'}
                      >
                        {formatCurrency(sale.outstanding_amount)}
                      </Text>
                    </Td>
                    <Td borderColor={borderColor} py={3}>
                      <Badge
                        colorScheme={getStatusColor(sale.status)}
                        variant="subtle"
                        px={2}
                        py={1}
                        borderRadius="md"
                        fontSize="xs"
                      >
                        {getStatusLabel(sale.status)}
                      </Badge>
                    </Td>
                    <Td borderColor={borderColor} py={3} textAlign="center" position="relative">
                      <Menu strategy="fixed" placement="bottom-end">
                        <MenuButton
                          as={IconButton}
                          icon={<FiMoreVertical />}
                          variant="ghost"
                          size="sm"
                          aria-label={t('common.actions')}
                          _hover={{ bg: hoverBg }}
                          data-testid={`actions-${sale.id}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            console.log('Menu button clicked for sale:', sale.id);
                          }}
                        />
                        <MenuList zIndex={9999} bg={tableBg} border="1px solid" borderColor={borderColor}>
                          {/* View Details - Always Available */}
                          <MenuItem
                            icon={<FiEye />}
                            onClick={() => {
                              console.log('View Details clicked for sale:', sale.id);
                              onViewDetails(sale);
                            }}
                          >
                            {t('common.buttons.viewDetails')}
                          </MenuItem>

                          {/* Duplicate Action */}
                          {onDuplicate && (
                            <MenuItem
                              icon={<FiCopy />}
                              onClick={() => {
                                console.log('Duplicate clicked for sale:', sale.id);
                                onDuplicate(sale);
                              }}
                            >
                              {t('common.duplicate')}
                            </MenuItem>
                          )}


                          {sale.status === 'DRAFT' && canEdit && onEdit && (
                            <MenuItem
                              icon={<FiEdit />}
                              onClick={() => {
                                console.log('Edit clicked for sale:', sale.id, 'canEdit:', canEdit);
                                onEdit(sale);
                              }}
                            >
                              {t('common.edit')}
                            </MenuItem>
                          )}
                          {sale.status === 'DRAFT' && canEdit && onConfirm && (
                            <MenuItem
                              icon={<FiCheck />}
                              onClick={() => {
                                console.log('Create Invoice clicked for sale:', sale.id);
                                onConfirm(sale);
                              }}
                            >
                              {t('sales.invoice')}
                            </MenuItem>
                          )}
                          {sale.status === 'INVOICED' && sale.outstanding_amount > 0 && canEdit && onPayment && (
                            <MenuItem icon={<FiDollarSign />} onClick={() => onPayment(sale)}>
                              {t('purchases.payment.recordPayment')}
                            </MenuItem>
                          )}
                          {canEdit && sale.status !== 'PAID' && sale.status !== 'CANCELLED' && onCancel && (
                            <MenuItem icon={<FiX />} onClick={() => onCancel(sale)}>
                              {t('common.cancel')}
                            </MenuItem>
                          )}
                          {onDownloadInvoice && (
                            <MenuItem
                              icon={<FiDownload />}
                              onClick={() => {
                                console.log('Download Invoice clicked for sale:', sale.id);
                                onDownloadInvoice(sale);
                              }}
                            >
                              {t('common.buttons.download')}
                            </MenuItem>
                          )}
                          {/* Create Receipt visible when fully paid */}
                          {(onCreateReceipt && (sale.status === 'PAID' || sale.outstanding_amount === 0)) && (
                            <MenuItem
                              icon={<FiDownload />}
                              onClick={() => {
                                console.log('Create Receipt clicked for sale:', sale.id);
                                onCreateReceipt(sale);
                              }}
                            >
                              {t('common.buttons.print')} (PDF)
                            </MenuItem>
                          )}
                          {canDelete && onDelete && (
                            <>
                              <MenuDivider />
                              <MenuItem
                                icon={<FiTrash2 />}
                                color="red.500"
                                onClick={() => {
                                  console.log('Delete clicked for sale:', sale.id, 'canDelete:', canDelete);
                                  onDelete(sale);
                                }}
                              >
                                {t('common.delete')}
                              </MenuItem>
                            </>
                          )}

                          {/* Fallback if no actions available */}
                          {!canEdit && !canDelete && !onDownloadInvoice && (
                            <>
                              <MenuDivider />
                              <MenuItem isDisabled fontSize="xs" color="gray.500">
                                {t('common.table.noData')}
                              </MenuItem>
                            </>
                          )}
                        </MenuList>
                      </Menu>
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </Box>
        )}
      </CardBody>
    </Card>
  );
};

export default EnhancedSalesTable;
