'use client';

import React, { useState, useEffect, useCallback } from 'react';
import {
  Box,
  VStack,
  HStack,
  FormControl,
  FormLabel,
  Input,
  Textarea,
  Button,
  Select,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  IconButton,
  Alert,
  AlertIcon,
  Badge,
  Card,
  CardHeader,
  CardBody,
  Flex,
  Text,
  Spinner,
  useToast,
  useDisclosure,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Divider,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  useColorModeValue
} from '@chakra-ui/react';
import {
  FiPlus,
  FiTrash2,
  FiSave,
  FiCheck,
  FiAlertTriangle,
  FiActivity,
  FiEye,
  FiRefreshCw
} from 'react-icons/fi';
import { useForm, useFieldArray } from 'react-hook-form';
import { ssotJournalService, CreateJournalRequest, SSOTJournalEntry } from '@/services/ssotJournalService';
import { formatCurrency } from '@/utils/formatters';

interface JournalLineFormData {
  account_id: number;
  description: string;
  debit_amount?: number;
  credit_amount?: number;
  quantity?: number;
  unit_price?: number;
}

interface JournalEntryFormData {
  entry_date: string;
  description: string;
  reference?: string;
  notes?: string;
  lines: JournalLineFormData[];
}

interface JournalEntryFormProps {
  initialData?: SSOTJournalEntry;
  onSave?: (entry: SSOTJournalEntry) => void;
  onCancel?: () => void;
  readOnly?: boolean;
  showRealTimeMonitor?: boolean;
}

// Sample accounts - in production, this would come from an API
const SAMPLE_ACCOUNTS = [
  { id: 1101, code: '1101', name: 'Kas', type: 'ASSET' },
  { id: 1102, code: '1102', name: 'Bank BCA', type: 'ASSET' },
  { id: 1103, code: '1103', name: 'Bank Mandiri', type: 'ASSET' },
  { id: 1201, code: '1201', name: 'Piutang Usaha', type: 'ASSET' },
  { id: 2101, code: '2101', name: 'Utang Usaha', type: 'LIABILITY' },
  { id: 2102, code: '2102', name: 'Utang Pajak', type: 'LIABILITY' },
  { id: 3101, code: '3101', name: 'Modal Pemilik', type: 'EQUITY' },
  { id: 4101, code: '4101', name: 'Pendapatan Penjualan', type: 'REVENUE' },
  { id: 5101, code: '5101', name: 'Harga Pokok Penjualan', type: 'EXPENSE' },
  { id: 5201, code: '5201', name: 'Beban Gaji', type: 'EXPENSE' }
];

const JournalEntryForm: React.FC<JournalEntryFormProps> = ({
  initialData,
  onSave,
  onCancel,
  readOnly = false,
  showRealTimeMonitor = true
}) => {
  const { register, control, handleSubmit, watch, setValue, formState: { errors, isSubmitting } } = useForm<JournalEntryFormData>({
    defaultValues: {
      entry_date: initialData?.entry_date || new Date().toISOString().split('T')[0],
      description: initialData?.description || '',
      reference: initialData?.reference || '',
      notes: initialData?.notes || '',
      lines: initialData?.lines?.map((line) => ({
        account_id: line.account_id,
        description: line.description,
        debit_amount: line.debit_amount || undefined,
        credit_amount: line.credit_amount || undefined,
        quantity: line.quantity || undefined,
        unit_price: line.unit_price || undefined
      })) || [{ account_id: 0, description: '', debit_amount: 0, credit_amount: 0 }]
    }
  });

  const { fields, append, remove } = useFieldArray({
    control,
    name: 'lines'
  });

  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();
  
  // Real-time monitoring removed - using standard refresh instead
  
  // Component state
  const [isLoading, setIsLoading] = useState(false);
  const [savedEntry, setSavedEntry] = useState<SSOTJournalEntry | null>(null);
  const [balanceChanges, setBalanceChanges] = useState<Map<number, number>>(new Map());
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [isConnected] = useState(false);
  const [updateCount] = useState(0);
  const [lastUpdateTime] = useState<Date | null>(null);

  // Color mode values
  const cardBg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const errorBg = useColorModeValue('red.50', 'red.900');
  const successBg = useColorModeValue('green.50', 'green.900');

  // Watch form values for real-time calculations
  const watchedLines = watch('lines');
  
  // Calculate totals
  const totalDebit = watchedLines?.reduce((sum, line) => sum + (line.debit_amount || 0), 0) || 0;
  const totalCredit = watchedLines?.reduce((sum, line) => sum + (line.credit_amount || 0), 0) || 0;
  const isBalanced = Math.abs(totalDebit - totalCredit) < 0.01;

  // Real-time monitoring removed - balance updates handled by standard refresh

  // Validation
  const validateJournalEntry = useCallback((): string[] => {
    const errors: string[] = [];
    
    if (!watchedLines || watchedLines.length === 0) {
      errors.push('At least one journal line is required');
    }
    
    if (!isBalanced) {
      errors.push(`Journal entry is not balanced. Debit: ${formatCurrency(totalDebit)}, Credit: ${formatCurrency(totalCredit)}`);
    }
    
    watchedLines?.forEach((line, index) => {
      if (!line.account_id || line.account_id === 0) {
        errors.push(`Line ${index + 1}: Account is required`);
      }
      
      if (!line.description?.trim()) {
        errors.push(`Line ${index + 1}: Description is required`);
      }
      
      if ((!line.debit_amount || line.debit_amount === 0) && (!line.credit_amount || line.credit_amount === 0)) {
        errors.push(`Line ${index + 1}: Either debit or credit amount is required`);
      }
      
      if (line.debit_amount && line.debit_amount > 0 && line.credit_amount && line.credit_amount > 0) {
        errors.push(`Line ${index + 1}: Cannot have both debit and credit amounts`);
      }
    });
    
    return errors;
  }, [watchedLines, isBalanced, totalDebit, totalCredit]);

  // Update validation errors in real-time
  useEffect(() => {
    const errors = validateJournalEntry();
    setValidationErrors(errors);
  }, [validateJournalEntry]);

  // Add new line
  const addLine = () => {
    append({
      account_id: 0,
      description: '',
      debit_amount: 0,
      credit_amount: 0
    });
  };

  // Remove line
  const removeLine = (index: number) => {
    if (fields.length > 1) {
      remove(index);
    }
  };

  // Get account info
  const getAccountInfo = (accountId: number) => {
    return SAMPLE_ACCOUNTS.find(acc => acc.id === accountId);
  };

  // Get current balance for account
  const getCurrentBalance = (accountId: number) => {
    return balanceChanges.get(accountId) || 0;
  };

  const toggleConnection = () => {};

  // Submit journal entry
  const onSubmit = async (data: JournalEntryFormData) => {
    try {
      setIsLoading(true);
      
      // Final validation
      const errors = validateJournalEntry();
      if (errors.length > 0) {
        toast({
          title: 'Validation Error',
          description: errors[0],
          status: 'error',
          duration: 5000,
          isClosable: true
        });
        return;
      }

      // Prepare data for API
      const journalRequest: CreateJournalRequest = {
        entry_date: data.entry_date,
        description: data.description,
        reference: data.reference,
        notes: data.notes,
        lines: data.lines.map(line => ({
          account_id: line.account_id,
          description: line.description,
          debit_amount: line.debit_amount || 0,
          credit_amount: line.credit_amount || 0,
          quantity: line.quantity,
          unit_price: line.unit_price
        }))
      };

      // Create journal entry
      const result = initialData?.id 
        ? await ssotJournalService.updateJournalEntry(initialData.id, journalRequest)
        : await ssotJournalService.createJournalEntry(journalRequest);

      setSavedEntry(result);
      
      toast({
        title: initialData ? 'Journal Entry Updated' : 'Journal Entry Created',
        description: `Entry ${result.entry_number} has been saved successfully`,
        status: 'success',
        duration: 5000,
        isClosable: true
      });

      if (onSave) {
        onSave(result);
      }

      // Show success modal
      onOpen();

    } catch (error) {
      console.error('Error saving journal entry:', error);
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to save journal entry',
        status: 'error',
        duration: 5000,
        isClosable: true
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box>
      <form onSubmit={handleSubmit(onSubmit)}>
        <VStack spacing={6} align="stretch">
          {/* Header Section */}
          <Card>
            <CardHeader>
              <Flex justify="space-between" align="center">
                <Text fontSize="lg" fontWeight="semibold">
                  {initialData ? 'Edit Journal Entry' : 'Create Journal Entry'}
                </Text>
                
                {showRealTimeMonitor && (
                  <HStack spacing={3}>
                    <Badge 
                      colorScheme={isConnected ? 'green' : 'red'} 
                      variant="subtle"
                      fontSize="xs"
                    >
                      {isConnected ? 'Live Updates' : 'Offline'}
                    </Badge>
                    <Button
                      size="sm"
                      variant="outline"
                      leftIcon={<FiActivity />}
                      onClick={toggleConnection}
                    >
                      {isConnected ? 'Disconnect' : 'Connect'}
                    </Button>
                  </HStack>
                )}
              </Flex>
            </CardHeader>
            
            <CardBody>
              <HStack spacing={4}>
                <FormControl isRequired>
                  <FormLabel>Entry Date</FormLabel>
                  <Input
                    type="date"
                    {...register('entry_date', { required: 'Entry date is required' })}
                    isReadOnly={readOnly}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>Description</FormLabel>
                  <Input
                    placeholder="Journal entry description"
                    {...register('description', { required: 'Description is required' })}
                    isReadOnly={readOnly}
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>Reference</FormLabel>
                  <Input
                    placeholder="Reference number"
                    {...register('reference')}
                    isReadOnly={readOnly}
                  />
                </FormControl>
              </HStack>

              <FormControl mt={4}>
                <FormLabel>Notes</FormLabel>
                <Textarea
                  placeholder="Additional notes (optional)"
                  {...register('notes')}
                  isReadOnly={readOnly}
                  rows={2}
                />
              </FormControl>
            </CardBody>
          </Card>

          {/* Journal Lines Section */}
          <Card>
            <CardHeader>
              <Flex justify="space-between" align="center">
                <Text fontSize="md" fontWeight="semibold">Journal Lines</Text>
                {!readOnly && (
                  <Button
                    size="sm"
                    leftIcon={<FiPlus />}
                    onClick={addLine}
                    colorScheme="blue"
                    variant="outline"
                  >
                    Add Line
                  </Button>
                )}
              </Flex>
            </CardHeader>
            
            <CardBody p={0}>
              <Table variant="simple">
                <Thead bg={headerBg}>
                  <Tr>
                    <Th>Account</Th>
                    <Th>Description</Th>
                    <Th width="120px">Debit</Th>
                    <Th width="120px">Credit</Th>
                    {showRealTimeMonitor && <Th width="120px">Current Balance</Th>}
                    {!readOnly && <Th width="60px">Actions</Th>}
                  </Tr>
                </Thead>
                <Tbody>
                  {fields.map((field, index) => {
                    const accountInfo = getAccountInfo(watchedLines?.[index]?.account_id || 0);
                    const currentBalance = getCurrentBalance(watchedLines?.[index]?.account_id || 0);
                    
                    return (
                      <Tr key={field.id}>
                        <Td>
                          <Select
                            {...register(`lines.${index}.account_id` as const, { 
                              required: 'Account is required',
                              valueAsNumber: true 
                            })}
                            placeholder="Select account"
                            isDisabled={readOnly}
                            size="sm"
                          >
                            {SAMPLE_ACCOUNTS.map(account => (
                              <option key={account.id} value={account.id}>
                                {account.code} - {account.name}
                              </option>
                            ))}
                          </Select>
                          {accountInfo && (
                            <Text fontSize="xs" color="gray.500" mt={1}>
                              {accountInfo.type}
                            </Text>
                          )}
                        </Td>
                        
                        <Td>
                          <Input
                            {...register(`lines.${index}.description` as const, { 
                              required: 'Description is required' 
                            })}
                            placeholder="Line description"
                            size="sm"
                            isReadOnly={readOnly}
                          />
                        </Td>
                        
                        <Td>
                          <Input
                            type="number"
                            step="0.01"
                            {...register(`lines.${index}.debit_amount` as const, { 
                              valueAsNumber: true 
                            })}
                            placeholder="0.00"
                            size="sm"
                            isReadOnly={readOnly}
                            textAlign="right"
                          />
                        </Td>
                        
                        <Td>
                          <Input
                            type="number"
                            step="0.01"
                            {...register(`lines.${index}.credit_amount` as const, { 
                              valueAsNumber: true 
                            })}
                            placeholder="0.00"
                            size="sm"
                            isReadOnly={readOnly}
                            textAlign="right"
                          />
                        </Td>

                        {showRealTimeMonitor && (
                          <Td>
                            <Text
                              fontSize="sm"
                              fontWeight="medium"
                              color="green.600"
                              textAlign="right"
                            >
                              {formatCurrency(Math.abs(currentBalance))}
                            </Text>
                          </Td>
                        )}
                        
                        {!readOnly && (
                          <Td>
                            <IconButton
                              aria-label="Remove line"
                              icon={<FiTrash2 />}
                              size="sm"
                              variant="ghost"
                              colorScheme="red"
                              onClick={() => removeLine(index)}
                              isDisabled={fields.length <= 1}
                            />
                          </Td>
                        )}
                      </Tr>
                    );
                  })}
                </Tbody>
              </Table>
            </CardBody>
          </Card>

          {/* Totals and Validation */}
          <Card>
            <CardBody>
              <HStack justify="space-between">
                <HStack spacing={8}>
                  <Stat>
                    <StatLabel>Total Debit</StatLabel>
                    <StatNumber color="blue.600">{formatCurrency(totalDebit)}</StatNumber>
                  </Stat>
                  
                  <Stat>
                    <StatLabel>Total Credit</StatLabel>
                    <StatNumber color="green.600">{formatCurrency(totalCredit)}</StatNumber>
                  </Stat>
                  
                  <Stat>
                    <StatLabel>Difference</StatLabel>
                    <StatNumber color={isBalanced ? 'green.600' : 'red.600'}>
                      {formatCurrency(Math.abs(totalDebit - totalCredit))}
                    </StatNumber>
                    <StatHelpText>
                      <Badge colorScheme={isBalanced ? 'green' : 'red'} variant="subtle">
                        {isBalanced ? 'Balanced' : 'Not Balanced'}
                      </Badge>
                    </StatHelpText>
                  </Stat>
                </HStack>

                {showRealTimeMonitor && isConnected && (
                  <VStack align="end" spacing={1}>
                    <HStack spacing={2}>
                      <FiActivity color="green" />
                      <Text fontSize="sm" color="green.600">Live Updates</Text>
                    </HStack>
                    <Text fontSize="xs" color="gray.500">
                      {updateCount} updates received
                    </Text>
                    {lastUpdateTime && (
                      <Text fontSize="xs" color="gray.500">
                        Last: {lastUpdateTime.toLocaleTimeString()}
                      </Text>
                    )}
                  </VStack>
                )}
              </HStack>
            </CardBody>
          </Card>

          {/* Validation Errors */}
          {validationErrors.length > 0 && (
            <Alert status="error">
              <AlertIcon />
              <VStack align="start" spacing={1}>
                <Text fontWeight="medium">Validation Errors:</Text>
                {validationErrors.map((error, index) => (
                  <Text key={index} fontSize="sm">• {error}</Text>
                ))}
              </VStack>
            </Alert>
          )}

          {/* Actions */}
          {!readOnly && (
            <HStack spacing={4} justify="end">
              {onCancel && (
                <Button variant="ghost" onClick={onCancel}>
                  Cancel
                </Button>
              )}
              
              <Button
                type="submit"
                colorScheme="blue"
                leftIcon={<FiSave />}
                isLoading={isLoading || isSubmitting}
                loadingText="Saving..."
                isDisabled={validationErrors.length > 0}
              >
                {initialData ? 'Update Entry' : 'Save Entry'}
              </Button>
            </HStack>
          )}
        </VStack>
      </form>

      {/* Success Modal */}
      <Modal isOpen={isOpen} onClose={onClose} size="md">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            <HStack>
              <FiCheck color="green" />
              <Text>Journal Entry Saved</Text>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody>
            {savedEntry && (
              <VStack spacing={4} align="stretch">
                <Alert status="success">
                  <AlertIcon />
                  Journal entry has been saved successfully!
                </Alert>
                
                <Box p={4} bg={successBg} borderRadius="md">
                  <VStack spacing={2} align="stretch">
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="medium">Entry Number:</Text>
                      <Text fontSize="sm">{savedEntry.entry_number}</Text>
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="medium">Status:</Text>
                      <Badge colorScheme="blue">{savedEntry.status}</Badge>
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="medium">Total Amount:</Text>
                      <Text fontSize="sm" fontWeight="medium">
                        {formatCurrency(savedEntry.total_debit)}
                      </Text>
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="medium">Balanced:</Text>
                      <Badge colorScheme={savedEntry.is_balanced ? 'green' : 'red'}>
                        {savedEntry.is_balanced ? 'Yes' : 'No'}
                      </Badge>
                    </HStack>
                  </VStack>
                </Box>
              </VStack>
            )}
          </ModalBody>
          <ModalFooter>
            <Button colorScheme="blue" onClick={onClose}>
              Close
            </Button>
          </ModalFooter>
        </ModalContent>
      </Modal>
    </Box>
  );
};

export default JournalEntryForm;
