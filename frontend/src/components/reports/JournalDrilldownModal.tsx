'use client';

import React, { useState, useEffect, useRef } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  Box,
  VStack,
  HStack,
  Text,
  Button,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Badge,
  IconButton,
  Tooltip,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Spinner,
  Select,
  Input,
  NumberInput,
  NumberInputField,
  Flex,
  Spacer,
  Divider,
  Card,
  CardHeader,
  CardBody,
  useColorModeValue,
  Stack,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  MenuDivider,
  useToast,
} from '@chakra-ui/react';
import {
  FiEye,
  FiDownload,
  FiFilter,
  FiX,
  FiChevronLeft,
  FiChevronRight,
  FiRefreshCw,
  FiFileText,
  FiFile,
  FiPrinter,
  FiActivity,
} from 'react-icons/fi';
import { useAuth } from '@/contexts/AuthContext';
import { formatCurrency } from '@/utils/formatters';
import { cleanCsvCell, cleanCsvContent } from '@/utils/exportText';

// Types
interface JournalEntry {
  id: number;
  code: string;
  description: string;
  reference: string;
  reference_type: string;
  entry_date: string;
  status: string;
  total_debit: number;
  total_credit: number;
  is_balanced: boolean;
  creator: {
    id: number;
    name: string;
  };
  journal_lines?: JournalLine[];
}

interface JournalLine {
  id: number;
  account_id: number;
  description: string;
  debit_amount: number;
  credit_amount: number;
  account: {
    id: number;
    code: string;
    name: string;
  };
}

interface JournalDrilldownRequest {
  account_codes?: string[];
  account_ids?: number[];  // Will be converted to uint in backend
  start_date: string;
  end_date: string;
  report_type?: string;
  line_item_name?: string;
  min_amount?: number;
  max_amount?: number;
  transaction_types?: string[];
  page: number;
  limit: number;
}

interface JournalDrilldownResponse {
  journal_entries: JournalEntry[];
  total: number;
  summary: {
    total_debit: number;
    total_credit: number;
    net_amount: number;
    entry_count: number;
    date_range_start: string;
    date_range_end: string;
    accounts_involved: string[];
  };
  metadata: {
    report_type: string;
    line_item_name: string;
    filter_criteria: string;
    generated_at: string;
  };
}

interface JournalDrilldownModalProps {
  isOpen: boolean;
  onClose: () => void;
  drilldownRequest: JournalDrilldownRequest;
  title?: string;
}

export const JournalDrilldownModal: React.FC<JournalDrilldownModalProps> = ({
  isOpen,
  onClose,
  drilldownRequest,
  title = 'Journal Entry Details',
}) => {
  const { token } = useAuth();
  const toast = useToast();
  const [loading, setLoading] = useState(false);
  const [data, setData] = useState<JournalDrilldownResponse | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [selectedEntry, setSelectedEntry] = useState<JournalEntry | null>(null);
  const [showFilters, setShowFilters] = useState(false);
  const [isConnectedToBalanceService] = useState(false);
  const [filters, setFilters] = useState({
    transaction_type: '',
    min_amount: '',
    max_amount: '',
  });
  
  // Pagination state
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage, setItemsPerPage] = useState(20);

  // Color mode values
  const bgColor = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const hoverBg = useColorModeValue('gray.50', 'gray.700');
  const scrollbarTrack = useColorModeValue('#f1f1f1', '#4a5568');
  const scrollbarThumb = useColorModeValue('#c1c1c1', '#718096');
  const scrollbarThumbHover = useColorModeValue('#a8a8a8', '#4a5568');

  useEffect(() => {
    if (isOpen && token) {
      fetchJournalEntries();
    }
  }, [isOpen, token, currentPage, itemsPerPage]);

  const fetchJournalEntries = async () => {
    if (!token) return;

    setLoading(true);
    setError(null);

    try {
      // Validate required drilldown request
      if (!drilldownRequest) {
        throw new Error('No drilldown request provided');
      }

      // Convert string dates to RFC3339 format for backend
      const convertToRFC3339 = (dateString: string): string => {
        if (!dateString || dateString.trim() === '') {
          console.warn('Empty date string provided, using current date');
          return new Date().toISOString();
        }
        
        // If already in ISO format, return as-is
        if (dateString.includes('T')) {
          return dateString;
        }
        
        // Convert YYYY-MM-DD to RFC3339 format
        const date = new Date(dateString + 'T00:00:00.000Z');
        if (isNaN(date.getTime())) {
          console.warn('Invalid date provided:', dateString, '- using current date');
          return new Date().toISOString();
        }
        return date.toISOString();
      };

      // Ensure account_ids are valid numbers (convert to uint compatible format)
      const validateAccountIds = (ids?: number[]): number[] | undefined => {
        if (!ids || !Array.isArray(ids)) return undefined;
        return ids.filter(id => Number.isInteger(id) && id >= 0);
      };

      const requestPayload = {
        ...drilldownRequest,
        start_date: convertToRFC3339(drilldownRequest.start_date),
        end_date: convertToRFC3339(drilldownRequest.end_date),
        account_ids: validateAccountIds(drilldownRequest.account_ids),
        page: currentPage,
        limit: itemsPerPage,
        ...(filters.min_amount !== '' ? { min_amount: Number(filters.min_amount) } : {}),
        ...(filters.max_amount !== '' ? { max_amount: Number(filters.max_amount) } : {}),
        ...(filters.transaction_type !== '' ? { transaction_types: [filters.transaction_type] } : {}),
      };

      console.log('📊 Journal Drilldown Request:', {
        ...requestPayload,
        // Mask sensitive data in logs if needed
        token: '***masked***'
      });

      const response = await fetch('/api/v1/journal-drilldown', {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          'Authorization': `Bearer ${token}`,
        },
        body: JSON.stringify(requestPayload),
      });

      if (!response.ok) {
        const contentType = response.headers.get('content-type');
        let errorMessage = `HTTP ${response.status}: ${response.statusText}`;
        
        try {
          if (contentType && contentType.includes('application/json')) {
            const errorData = await response.json();
            errorMessage = errorData.message || errorData.error || errorMessage;
            console.error('❌ Journal Drilldown JSON Error:', errorData);
          } else {
            const errorText = await response.text();
            errorMessage = errorText || errorMessage;
            console.error('❌ Journal Drilldown Text Error:', errorText);
          }
        } catch (parseError) {
          console.error('❌ Error parsing error response:', parseError);
        }
        
        throw new Error(`Failed to fetch journal entries: ${errorMessage}`);
      }

      const result = await response.json();
      console.log('✅ Journal Drilldown Response:', result);
      
      // Validate response structure
      if (!result || !result.data) {
        console.warn('Invalid response structure:', result);
        throw new Error('Invalid response format from server');
      }
      
      setData(result.data);
    } catch (err) {
      setError(err instanceof Error ? err.message : 'Unknown error occurred');
    } finally {
      setLoading(false);
    }
  };

  const handleApplyFilters = () => {
    setCurrentPage(1);
    fetchJournalEntries();
  };

  const handleClearFilters = () => {
    setFilters({
      transaction_type: '',
      min_amount: '',
      max_amount: '',
    });
    setCurrentPage(1);
    fetchJournalEntries();
  };

  const handleViewEntry = async (entryId: number) => {
    if (!token) return;

    try {
      const response = await fetch(`/api/v1/journal-drilldown/entries/${entryId}`, {
        headers: {
          'Authorization': `Bearer ${token}`,
        },
      });

      if (!response.ok) {
        throw new Error('Failed to fetch journal entry details');
      }

      const result = await response.json();
      setSelectedEntry(result.data);
    } catch (err) {
      console.error('Error fetching journal entry details:', err);
    }
  };

  const handleExportData = (format: 'csv' | 'excel' | 'pdf' = 'csv') => {
    if (!data) return;

    switch (format) {
      case 'csv':
        exportAsCSV();
        break;
      case 'excel':
        exportAsExcel();
        break;
      case 'pdf':
        exportAsPDF();
        break;
    }
  };

  const exportAsCSV = () => {
    if (!data) return;

    // Create comprehensive CSV content with summary
    const headers = ['Date', 'Code', 'Description', 'Reference', 'Type', 'Debit', 'Credit', 'Status', 'Creator', 'Balanced'];
    const formatCsvCell = (value: unknown) => `"${cleanCsvCell(value).replace(/"/g, '""')}"`;
    const csvContent = cleanCsvContent([
      // Add summary section
      'Journal Entry Drilldown Report',
      `Generated: ${new Date().toLocaleString()}`,
      `Period: ${data.metadata?.line_item_name || 'N/A'}`,
      `Date Range: ${data.summary?.date_range_start || ''} to ${data.summary?.date_range_end || ''}`,
      `Total Entries: ${data.summary?.entry_count || 0}`,
      `Total Debit: ${data.summary?.total_debit || 0}`,
      `Total Credit: ${data.summary?.total_credit || 0}`,
      `Net Amount: ${data.summary?.net_amount || 0}`,
      '',
      headers.map(formatCsvCell).join(','),
      ...data.journal_entries.map(entry => [
        new Date(entry.entry_date).toLocaleDateString(),
        entry.code,
        entry.description,
        entry.reference,
        entry.reference_type,
        entry.total_debit.toString(),
        entry.total_credit.toString(),
        entry.status,
        entry.creator?.name || 'Unknown',
        entry.is_balanced ? 'Yes' : 'No'
      ].map(formatCsvCell).join(','))
    ].join('\n'));

    // Create and download file
    const blob = new Blob([csvContent], { type: 'text/csv;charset=utf-8;' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `journal-drilldown-${new Date().toISOString().split('T')[0]}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  };

  const exportAsExcel = () => {
    if (!data) return;

    // Create Excel-compatible TSV content
    const headers = ['Date', 'Code', 'Description', 'Reference', 'Type', 'Debit', 'Credit', 'Status', 'Creator', 'Balanced'];
    const tsvContent = [
      // Summary section
      'Journal Entry Drilldown Report\t\t\t\t\t\t\t\t\t',
      `Generated:\t${new Date().toLocaleString()}\t\t\t\t\t\t\t\t`,
      `Period:\t${data.metadata?.line_item_name || 'N/A'}\t\t\t\t\t\t\t\t`,
      `Date Range:\t${data.summary?.date_range_start || ''} to ${data.summary?.date_range_end || ''}\t\t\t\t\t\t\t\t`,
      `Total Entries:\t${data.summary?.entry_count || 0}\t\t\t\t\t\t\t\t`,
      `Total Debit:\t${data.summary?.total_debit || 0}\t\t\t\t\t\t\t\t`,
      `Total Credit:\t${data.summary?.total_credit || 0}\t\t\t\t\t\t\t\t`,
      `Net Amount:\t${data.summary?.net_amount || 0}\t\t\t\t\t\t\t\t`,
      '\t\t\t\t\t\t\t\t\t',
      headers.join('\t'),
      ...data.journal_entries.map(entry => [
        new Date(entry.entry_date).toLocaleDateString(),
        entry.code,
        entry.description.replace(/\t/g, ' '), // Replace tabs with spaces
        entry.reference,
        entry.reference_type,
        entry.total_debit,
        entry.total_credit,
        entry.status,
        entry.creator?.name || 'Unknown',
        entry.is_balanced ? 'Yes' : 'No'
      ].join('\t'))
    ].join('\n');

    // Create and download file as Excel-compatible format
    const blob = new Blob([tsvContent], { type: 'application/vnd.ms-excel;charset=utf-8;' });
    const url = window.URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `journal-drilldown-${new Date().toISOString().split('T')[0]}.xls`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    window.URL.revokeObjectURL(url);
  };

  const exportAsPDF = async () => {
    if (!data) return;

    try {
      // Create a comprehensive HTML content for PDF generation
      const htmlContent = `
        <!DOCTYPE html>
        <html>
        <head>
          <meta charset="utf-8">
          <title>Journal Entry Drilldown Report</title>
          <style>
            body { font-family: Arial, sans-serif; margin: 20px; }
            .header { text-align: center; margin-bottom: 30px; }
            .summary { background-color: #f5f5f5; padding: 15px; margin-bottom: 20px; border-radius: 5px; }
            .summary-row { display: flex; justify-content: space-between; margin-bottom: 5px; }
            table { width: 100%; border-collapse: collapse; margin-top: 20px; }
            th, td { border: 1px solid #ddd; padding: 8px; text-align: left; font-size: 12px; }
            th { background-color: #f2f2f2; font-weight: bold; }
            .amount { text-align: right; }
            .status-posted { color: green; font-weight: bold; }
            .status-draft { color: orange; }
            .status-reversed { color: red; }
            .balanced-yes { color: green; }
            .balanced-no { color: red; }
            .total-row { background-color: #e8f4fd; font-weight: bold; }
            @media print { 
              body { margin: 0; }
              .header { page-break-after: avoid; }
            }
          </style>
        </head>
        <body>
          <div class="header">
            <h1>Journal Entry Drilldown Report</h1>
            <p><strong>${data.metadata?.line_item_name || 'Journal Entries'}</strong></p>
            <p>Generated on ${new Date().toLocaleDateString()} at ${new Date().toLocaleTimeString()}</p>
          </div>
          
          <div class="summary">
            <h3>Summary</h3>
            <div class="summary-row">
              <span>Date Range:</span>
              <span>${data.summary?.date_range_start || ''} to ${data.summary?.date_range_end || ''}</span>
            </div>
            <div class="summary-row">
              <span>Total Entries:</span>
              <span>${data.summary?.entry_count || 0}</span>
            </div>
            <div class="summary-row">
              <span>Total Debit:</span>
              <span class="amount">${formatCurrency(data.summary?.total_debit || 0)}</span>
            </div>
            <div class="summary-row">
              <span>Total Credit:</span>
              <span class="amount">${formatCurrency(data.summary?.total_credit || 0)}</span>
            </div>
            <div class="summary-row">
              <span>Net Amount:</span>
              <span class="amount">${formatCurrency(data.summary?.net_amount || 0)}</span>
            </div>
            <div class="summary-row">
              <span>Accounts Involved:</span>
              <span>${data.summary?.accounts_involved?.join(', ') || 'N/A'}</span>
            </div>
          </div>
          
          <table>
            <thead>
              <tr>
                <th>Date</th>
                <th>Code</th>
                <th>Description</th>
                <th>Reference</th>
                <th>Type</th>
                <th>Debit</th>
                <th>Credit</th>
                <th>Status</th>
                <th>Creator</th>
                <th>Balanced</th>
              </tr>
            </thead>
            <tbody>
              ${data.journal_entries.map(entry => `
                <tr>
                  <td>${new Date(entry.entry_date).toLocaleDateString()}</td>
                  <td>${entry.code}</td>
                  <td>${entry.description}</td>
                  <td>${entry.reference}</td>
                  <td>${entry.reference_type}</td>
                  <td class="amount">${formatCurrency(entry.total_debit)}</td>
                  <td class="amount">${formatCurrency(entry.total_credit)}</td>
                  <td class="status-${entry.status.toLowerCase()}">${entry.status}</td>
                  <td>${entry.creator?.name || 'Unknown'}</td>
                  <td class="balanced-${entry.is_balanced ? 'yes' : 'no'}">${entry.is_balanced ? 'Yes' : 'No'}</td>
                </tr>
              `).join('')}
              <tr class="total-row">
                <td colspan="5"><strong>TOTALS</strong></td>
                <td class="amount"><strong>${formatCurrency(data.summary?.total_debit || 0)}</strong></td>
                <td class="amount"><strong>${formatCurrency(data.summary?.total_credit || 0)}</strong></td>
                <td colspan="3"><strong>Net: ${formatCurrency(data.summary?.net_amount || 0)}</strong></td>
              </tr>
            </tbody>
          </table>
          
          <div style="margin-top: 30px; font-size: 10px; color: #666; text-align: center;">
            <p>Report generated by Accounting System | ${window.location.origin}</p>
            <p>Filter Criteria: ${data.metadata?.filter_criteria || 'Default'}</p>
          </div>
        </body>
        </html>
      `;

      // Create a new window for printing
      const printWindow = window.open('', '_blank');
      if (printWindow) {
        printWindow.document.write(htmlContent);
        printWindow.document.close();
        
        // Wait for content to load, then trigger print
        printWindow.onload = () => {
          setTimeout(() => {
            printWindow.print();
            printWindow.close();
          }, 500);
        };
      } else {
        // Fallback: create a blob and download as HTML file
        const blob = new Blob([htmlContent], { type: 'text/html' });
        const url = window.URL.createObjectURL(blob);
        const link = document.createElement('a');
        link.href = url;
        link.download = `journal-drilldown-${new Date().toISOString().split('T')[0]}.html`;
        document.body.appendChild(link);
        link.click();
        document.body.removeChild(link);
        window.URL.revokeObjectURL(url);
      }
      
    } catch (error) {
      console.error('PDF export failed:', error);
      // Fallback to CSV export
      exportAsCSV();
    }
  };

  const getStatusColor = (status: string) => {
    switch (status) {
      case 'POSTED': return 'green';
      case 'DRAFT': return 'yellow';
      case 'REVERSED': return 'red';
      default: return 'gray';
    }
  };

  const getTransactionTypeColor = (type: string) => {
    switch (type) {
      case 'SALE': return 'blue';
      case 'PURCHASE': return 'orange';
      case 'PAYMENT': return 'purple';
      case 'CASH_BANK': return 'teal';
      case 'PREPAID_AMORTIZATION': return 'purple';
      case 'MANUAL': return 'gray';
      default: return 'gray';
    }
  };

  const totalPages = data ? Math.ceil(data.total / itemsPerPage) : 0;

  if (selectedEntry) {
    return (
      <Modal isOpen={isOpen} onClose={onClose} size="6xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            <HStack>
              <IconButton
                aria-label="Back to list"
                icon={<FiChevronLeft />}
                size="sm"
                variant="ghost"
                onClick={() => setSelectedEntry(null)}
              />
              <Text>Journal Entry Details: {selectedEntry.code}</Text>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody 
            maxHeight="70vh"
            overflowY="auto"
            sx={{
              '&::-webkit-scrollbar': {
                width: '8px',
              },
              '&::-webkit-scrollbar-track': {
                background: scrollbarTrack,
                borderRadius: '4px',
              },
              '&::-webkit-scrollbar-thumb': {
                background: scrollbarThumb,
                borderRadius: '4px',
              },
              '&::-webkit-scrollbar-thumb:hover': {
                background: scrollbarThumbHover,
              },
            }}
          >
            <VStack spacing={6} align="stretch">
              {/* Entry Header Information */}
              <Card>
                <CardHeader>
                  <Text fontSize="lg" fontWeight="bold">Entry Information</Text>
                </CardHeader>
                <CardBody>
                  <Stack spacing={4}>
                    <HStack justify="space-between">
                      <Text><strong>Code:</strong> {selectedEntry.code}</Text>
                      <Badge colorScheme={getStatusColor(selectedEntry.status)}>
                        {selectedEntry.status}
                      </Badge>
                    </HStack>
                    <Text><strong>Description:</strong> {selectedEntry.description}</Text>
                    <HStack>
                      <Text><strong>Date:</strong> {new Date(selectedEntry.entry_date).toLocaleDateString()}</Text>
                      <Text><strong>Reference:</strong> {selectedEntry.reference}</Text>
                      <Badge colorScheme={getTransactionTypeColor(selectedEntry.reference_type)}>
                        {selectedEntry.reference_type}
                      </Badge>
                    </HStack>
                    <Text><strong>Created by:</strong> {selectedEntry.creator.name}</Text>
                  </Stack>
                </CardBody>
              </Card>

              {/* Journal Lines */}
              {selectedEntry.journal_lines && selectedEntry.journal_lines.length > 0 && (
                <Card>
                  <CardHeader>
                    <Text fontSize="lg" fontWeight="bold">Journal Lines</Text>
                  </CardHeader>
                  <CardBody>
                    <Box 
                      overflowX="auto"
                      overflowY="auto"
                      maxHeight="300px"
                      sx={{
                        '&::-webkit-scrollbar': {
                          width: '8px',
                          height: '8px',
                        },
                        '&::-webkit-scrollbar-track': {
                          background: scrollbarTrack,
                          borderRadius: '4px',
                        },
                        '&::-webkit-scrollbar-thumb': {
                          background: scrollbarThumb,
                          borderRadius: '4px',
                        },
                        '&::-webkit-scrollbar-thumb:hover': {
                          background: scrollbarThumbHover,
                        },
                        '&::-webkit-scrollbar-corner': {
                          background: scrollbarTrack,
                        },
                      }}
                    >
                      <Table size="sm">
                        <Thead>
                          <Tr>
                            <Th>Account</Th>
                            <Th>Description</Th>
                            <Th isNumeric>Debit</Th>
                            <Th isNumeric>Credit</Th>
                          </Tr>
                        </Thead>
                        <Tbody>
                          {selectedEntry.journal_lines.map((line) => (
                            <Tr key={line.id}>
                              <Td>
                                <VStack align="start" spacing={0}>
                                  <Text fontSize="sm" fontWeight="medium">
                                    {line.account.code}
                                  </Text>
                                  <Text fontSize="xs" color="gray.600">
                                    {line.account.name}
                                  </Text>
                                </VStack>
                              </Td>
                              <Td>{line.description}</Td>
                              <Td isNumeric color={line.debit_amount > 0 ? 'green.600' : undefined}>
                                {line.debit_amount > 0 ? formatCurrency(line.debit_amount) : '-'}
                              </Td>
                              <Td isNumeric color={line.credit_amount > 0 ? 'red.600' : undefined}>
                                {line.credit_amount > 0 ? formatCurrency(line.credit_amount) : '-'}
                              </Td>
                            </Tr>
                          ))}
                        </Tbody>
                      </Table>
                    </Box>
                  </CardBody>
                </Card>
              )}

              {/* Entry Totals */}
              <Card>
                <CardBody>
                  <HStack justify="space-between">
                    <Text fontWeight="bold">Total Debit: {formatCurrency(selectedEntry.total_debit)}</Text>
                    <Text fontWeight="bold">Total Credit: {formatCurrency(selectedEntry.total_credit)}</Text>
                    <Badge colorScheme={selectedEntry.is_balanced ? 'green' : 'red'}>
                      {selectedEntry.is_balanced ? 'BALANCED' : 'UNBALANCED'}
                    </Badge>
                  </HStack>
                </CardBody>
              </Card>
            </VStack>
          </ModalBody>
          <ModalFooter>
            <Button onClick={() => setSelectedEntry(null)}>Back to List</Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    );
  }

  return (
    <Modal isOpen={isOpen} onClose={onClose} size="6xl">
      <ModalOverlay />
      <ModalContent bg={bgColor}>
        <ModalHeader borderBottomWidth="1px" borderColor={borderColor}>
          <HStack justify="space-between">
            <VStack align="start" spacing={1}>
              <Text fontSize="lg" fontWeight="bold">{title}</Text>
              {data?.metadata && (
                <Text fontSize="sm" color="gray.600">
                  {data.metadata.line_item_name} • {data.metadata.filter_criteria}
                </Text>
              )}
            </VStack>
            <HStack>
              <Tooltip label="Refresh data">
                <IconButton
                  aria-label="Refresh"
                  icon={<FiRefreshCw />}
                  size="sm"
                  variant="ghost"
                  onClick={fetchJournalEntries}
                  isLoading={loading}
                />
              </Tooltip>
              <Tooltip label="Toggle filters">
                <IconButton
                  aria-label="Toggle filters"
                  icon={<FiFilter />}
                  size="sm"
                  variant={showFilters ? 'solid' : 'ghost'}
                  colorScheme={showFilters ? 'blue' : undefined}
                  onClick={() => setShowFilters(!showFilters)}
                />
              </Tooltip>
              {data && (
                <Menu>
                  <MenuButton
                    as={IconButton}
                    aria-label="Export options"
                    icon={<FiDownload />}
                    size="sm"
                    variant="ghost"
                    colorScheme="green"
                  />
                  <MenuList>
                    <MenuItem icon={<FiFileText />} onClick={() => handleExportData('csv')}>
                      Export as CSV
                    </MenuItem>
                    <MenuItem icon={<FiFile />} onClick={() => handleExportData('excel')}>
                      Export as Excel
                    </MenuItem>
                    <MenuDivider />
                    <MenuItem icon={<FiPrinter />} onClick={() => handleExportData('pdf')}>
                      Print/PDF Report
                    </MenuItem>
                  </MenuList>
                </Menu>
              )}
            </HStack>
          </HStack>
        </ModalHeader>

        <ModalCloseButton />

        <ModalBody 
          maxHeight="70vh"
          overflowY="auto"
          sx={{
            '&::-webkit-scrollbar': {
              width: '8px',
            },
            '&::-webkit-scrollbar-track': {
              background: scrollbarTrack,
              borderRadius: '4px',
            },
            '&::-webkit-scrollbar-thumb': {
              background: scrollbarThumb,
              borderRadius: '4px',
            },
            '&::-webkit-scrollbar-thumb:hover': {
              background: scrollbarThumbHover,
            },
          }}
        >
          <VStack spacing={4} align="stretch">
            {/* Summary Information */}
            {data?.summary && (
              <Card>
                <CardBody>
                  <HStack justify="space-between" wrap="wrap">
                    <VStack align="start" spacing={1}>
                      <Text fontSize="xs" color="gray.600">TOTAL ENTRIES</Text>
                      <Text fontSize="lg" fontWeight="bold">{data.summary.entry_count.toLocaleString()}</Text>
                    </VStack>
                    <VStack align="start" spacing={1}>
                      <Text fontSize="xs" color="gray.600">TOTAL DEBIT</Text>
                      <Text fontSize="lg" fontWeight="bold" color="green.600">
                        {formatCurrency(data.summary.total_debit)}
                      </Text>
                    </VStack>
                    <VStack align="start" spacing={1}>
                      <Text fontSize="xs" color="gray.600">TOTAL CREDIT</Text>
                      <Text fontSize="lg" fontWeight="bold" color="red.600">
                        {formatCurrency(data.summary.total_credit)}
                      </Text>
                    </VStack>
                    <VStack align="start" spacing={1}>
                      <Text fontSize="xs" color="gray.600">NET AMOUNT</Text>
                      <Text fontSize="lg" fontWeight="bold" color={data.summary.net_amount >= 0 ? 'green.600' : 'red.600'}>
                        {formatCurrency(data.summary.net_amount)}
                      </Text>
                    </VStack>
                  </HStack>
                </CardBody>
              </Card>
            )}

            {/* Filters */}
            {showFilters && (
              <Card>
                <CardHeader>
                  <HStack justify="space-between">
                    <Text fontSize="md" fontWeight="semibold">Filters</Text>
                    <IconButton
                      aria-label="Close filters"
                      icon={<FiX />}
                      size="sm"
                      variant="ghost"
                      onClick={() => setShowFilters(false)}
                    />
                  </HStack>
                </CardHeader>
                <CardBody>
                  <HStack spacing={4} wrap="wrap">
                    <Box minW="200px">
                      <Text fontSize="sm" mb={2}>Transaction Type</Text>
                      <Select
                        value={filters.transaction_type}
                        onChange={(e) => setFilters({...filters, transaction_type: e.target.value})}
                        size="sm"
                      >
                        <option value="">All Types</option>
                        <option value="SALE">Sale</option>
                        <option value="PURCHASE">Purchase</option>
                        <option value="PAYMENT">Payment</option>
                        <option value="CASH_BANK">Cash/Bank</option>
                        <option value="MANUAL">Manual</option>
                      </Select>
                    </Box>
                    <Box minW="150px">
                      <Text fontSize="sm" mb={2}>Min Amount</Text>
                      <NumberInput size="sm">
                        <NumberInputField
                          value={filters.min_amount}
                          onChange={(e) => setFilters({...filters, min_amount: e.target.value})}
                          placeholder="0.00"
                        />
                      </NumberInput>
                    </Box>
                    <Box minW="150px">
                      <Text fontSize="sm" mb={2}>Max Amount</Text>
                      <NumberInput size="sm">
                        <NumberInputField
                          value={filters.max_amount}
                          onChange={(e) => setFilters({...filters, max_amount: e.target.value})}
                          placeholder="0.00"
                        />
                      </NumberInput>
                    </Box>
                    <VStack>
                      <Button size="sm" colorScheme="blue" onClick={handleApplyFilters}>
                        Apply Filters
                      </Button>
                      <Button size="sm" variant="ghost" onClick={handleClearFilters}>
                        Clear
                      </Button>
                    </VStack>
                  </HStack>
                </CardBody>
              </Card>
            )}

            {/* Loading State */}
            {loading && (
              <Box textAlign="center" py={8}>
                <Spinner size="lg" />
                <Text mt={4}>Loading journal entries...</Text>
              </Box>
            )}

            {/* Error State */}
            {error && (
              <Alert status="error">
                <AlertIcon />
                <AlertTitle>Error!</AlertTitle>
                <AlertDescription>{error}</AlertDescription>
              </Alert>
            )}

            {/* Data Table */}
            {data && data.journal_entries.length > 0 && (
              <Card>
                <CardBody p={0}>
                  <Box 
                    overflowX="auto" 
                    overflowY="auto"
                    maxHeight="400px"
                    sx={{
                      '&::-webkit-scrollbar': {
                        width: '8px',
                        height: '8px',
                      },
                      '&::-webkit-scrollbar-track': {
                        background: scrollbarTrack,
                        borderRadius: '4px',
                      },
                      '&::-webkit-scrollbar-thumb': {
                        background: scrollbarThumb,
                        borderRadius: '4px',
                      },
                      '&::-webkit-scrollbar-thumb:hover': {
                        background: scrollbarThumbHover,
                      },
                      '&::-webkit-scrollbar-corner': {
                        background: scrollbarTrack,
                      },
                    }}
                  >
                    <Table size="sm">
                      <Thead bg={headerBg}>
                        <Tr>
                          <Th>Date</Th>
                          <Th>Code</Th>
                          <Th>Description</Th>
                          <Th>Reference</Th>
                          <Th>Type</Th>
                          <Th isNumeric>Debit</Th>
                          <Th isNumeric>Credit</Th>
                          <Th>Status</Th>
                          <Th>Actions</Th>
                        </Tr>
                      </Thead>
                      <Tbody>
                        {data.journal_entries.map((entry) => (
                          <Tr key={entry.id} _hover={{ bg: hoverBg }}>
                            <Td>
                              <Text fontSize="sm">
                                {new Date(entry.entry_date).toLocaleDateString()}
                              </Text>
                            </Td>
                            <Td>
                              <Text fontSize="sm" fontFamily="mono">
                                {entry.code}
                              </Text>
                            </Td>
                            <Td maxW="300px">
                              <Text fontSize="sm" noOfLines={2}>
                                {entry.description}
                              </Text>
                            </Td>
                            <Td>
                              <Text fontSize="sm">{entry.reference}</Text>
                            </Td>
                            <Td>
                              <Badge colorScheme={getTransactionTypeColor(entry.reference_type)} size="sm">
                                {entry.reference_type}
                              </Badge>
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" color="green.600" fontWeight="medium">
                                {formatCurrency(entry.total_debit)}
                              </Text>
                            </Td>
                            <Td isNumeric>
                              <Text fontSize="sm" color="red.600" fontWeight="medium">
                                {formatCurrency(entry.total_credit)}
                              </Text>
                            </Td>
                            <Td>
                              <Badge colorScheme={getStatusColor(entry.status)} size="sm">
                                {entry.status}
                              </Badge>
                            </Td>
                            <Td>
                              <Tooltip label="View details">
                                <IconButton
                                  aria-label="View entry"
                                  icon={<FiEye />}
                                  size="sm"
                                  variant="ghost"
                                  onClick={() => handleViewEntry(entry.id)}
                                />
                              </Tooltip>
                            </Td>
                          </Tr>
                        ))}
                      </Tbody>
                    </Table>
                  </Box>
                </CardBody>
              </Card>
            )}

            {/* Empty State */}
            {data && data.journal_entries.length === 0 && (
              <Box textAlign="center" py={8}>
                <Text fontSize="lg" color="gray.500">No journal entries found</Text>
                <Text fontSize="sm" color="gray.400">
                  Try adjusting your filters or date range
                </Text>
              </Box>
            )}

            {/* Pagination */}
            {data && data.total > itemsPerPage && (
              <Card>
                <CardBody>
                  <HStack justify="space-between" align="center">
                    <HStack>
                      <Text fontSize="sm" color="gray.600">
                        Showing {((currentPage - 1) * itemsPerPage) + 1} to {Math.min(currentPage * itemsPerPage, data.total)} of {data.total} entries
                      </Text>
                      <Select
                        value={itemsPerPage}
                        onChange={(e) => setItemsPerPage(Number(e.target.value))}
                        size="sm"
                        w="auto"
                      >
                        <option value={20}>20 per page</option>
                        <option value={50}>50 per page</option>
                        <option value={100}>100 per page</option>
                      </Select>
                    </HStack>
                    <HStack>
                      <IconButton
                        aria-label="Previous page"
                        icon={<FiChevronLeft />}
                        size="sm"
                        isDisabled={currentPage <= 1}
                        onClick={() => setCurrentPage(currentPage - 1)}
                      />
                      <Text fontSize="sm">
                        Page {currentPage} of {totalPages}
                      </Text>
                      <IconButton
                        aria-label="Next page"
                        icon={<FiChevronRight />}
                        size="sm"
                        isDisabled={currentPage >= totalPages}
                        onClick={() => setCurrentPage(currentPage + 1)}
                      />
                    </HStack>
                  </HStack>
                </CardBody>
              </Card>
            )}
          </VStack>
        </ModalBody>

        <ModalFooter>
          <Button onClick={onClose}>Close</Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default JournalDrilldownModal;
