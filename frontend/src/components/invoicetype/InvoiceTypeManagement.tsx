'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  useToast,
  IconButton,
  Text,
  Badge,
  Flex,
  Heading,
  useColorModeValue,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  FormControl,
  FormLabel,
  Input,
  FormErrorMessage,
  Switch,
  VStack,
  HStack,
  Alert,
  AlertIcon,
  AlertDescription,
  AlertTitle,
  useDisclosure
} from '@chakra-ui/react';
import { FiPlus, FiEdit, FiTrash2, FiSave, FiX, FiSettings, FiLock, FiHash } from 'react-icons/fi';
import { useForm } from 'react-hook-form';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import invoiceTypeService, { InvoiceType, InvoiceTypeCreateRequest, InvoiceTypeUpdateRequest, InvoiceCounter, ResetCounterRequest } from '@/services/invoiceTypeService';
import ErrorAlert, { ErrorDetail } from '@/components/common/ErrorAlert';
import ErrorHandler, { ParsedValidationError } from '@/utils/errorHandler';

interface FormData {
  name: string;
  code: string;
  is_active: boolean;
}

interface CounterFormData {
  year: number;
  counter: number;
}

const InvoiceTypeManagement: React.FC = () => {
  const { user } = useAuth();
  const settingsPermissions = useModulePermissions('settings');
  const salesPermissions = useModulePermissions('sales');
  
  // Check if user has settings permissions (full access) or sales permissions (read-only)
  const hasSettingsAccess = settingsPermissions.canView;
  const hasSalesAccess = salesPermissions.canView;
  const hasAnyAccess = hasSettingsAccess || hasSalesAccess;
  
  // Permissions for invoice types management
  const canView = hasAnyAccess;
  const canCreate = hasSettingsAccess || (user?.role === 'admin' || user?.role === 'finance' || user?.role === 'director');
  const canEdit = hasSettingsAccess || (user?.role === 'admin' || user?.role === 'finance' || user?.role === 'director');
  const canDelete = hasSettingsAccess && (user?.role === 'admin');
  
  const [invoiceTypes, setInvoiceTypes] = useState<InvoiceType[]>([]);
  const [loading, setLoading] = useState(true);
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [editingType, setEditingType] = useState<InvoiceType | null>(null);
  const [validationError, setValidationError] = useState<ParsedValidationError | null>(null);
  const [permissionError, setPermissionError] = useState<string | null>(null);
  const [isReadOnlyMode, setIsReadOnlyMode] = useState(false);
  const [selectedTypeForCounter, setSelectedTypeForCounter] = useState<InvoiceType | null>(null);
  const [counterHistory, setCounterHistory] = useState<InvoiceCounter[]>([]);
  const [loadingCounter, setLoadingCounter] = useState(false);
  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();
  const { isOpen: isCounterModalOpen, onOpen: onCounterModalOpen, onClose: onCounterModalClose } = useDisclosure();

  // Color mode values
  const bg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headingColor = useColorModeValue('gray.700', 'gray.200');
  const textColor = useColorModeValue('gray.600', 'gray.400');
  const counterHistoryItemBg = useColorModeValue('gray.50', 'gray.700');

  const {
    register,
    handleSubmit,
    reset,
    formState: { errors }
  } = useForm<FormData>();

  const {
    register: registerCounter,
    handleSubmit: handleSubmitCounter,
    reset: resetCounter,
    setValue: setCounterValue,
    formState: { errors: counterErrors }
  } = useForm<CounterFormData>();

  useEffect(() => {
    loadInvoiceTypes();
  }, []);

  const loadInvoiceTypes = async () => {
    try {
      setLoading(true);
      console.log('InvoiceTypeManagement: Loading invoice types...');
      
      // Try to use the active endpoint first for users with only sales permissions
      let types: InvoiceType[] = [];
      if (!hasSettingsAccess && hasSalesAccess) {
        console.log('InvoiceTypeManagement: User has sales access only, using active endpoint');
        types = await invoiceTypeService.getActiveInvoiceTypes();
        setIsReadOnlyMode(true);
      } else {
        console.log('InvoiceTypeManagement: User has settings access, using full endpoint');
        types = await invoiceTypeService.getInvoiceTypes();
        setIsReadOnlyMode(false);
      }
      
      console.log('InvoiceTypeManagement: Received invoice types:', types);
      setInvoiceTypes(types || []);
    } catch (error: any) {
      console.error('InvoiceTypeManagement: Error loading invoice types:', error);
      
      // Handle permission errors specifically
      if (error.response?.status === 403) {
        // If full access failed but user has sales access, try the active endpoint
        if (!hasSettingsAccess && hasSalesAccess && !isReadOnlyMode) {
          console.log('InvoiceTypeManagement: Full access denied, trying active endpoint for sales users');
          try {
            const activeTypes = await invoiceTypeService.getActiveInvoiceTypes();
            setInvoiceTypes(activeTypes || []);
            setIsReadOnlyMode(true);
            toast({
              title: 'Limited Access',
              description: 'You have read-only access to invoice types. Contact administrator for full management access.',
              status: 'warning',
              duration: 8000,
              isClosable: true,
            });
            return;
          } catch (activeError) {
            console.error('Active endpoint also failed:', activeError);
          }
        }
        
        const permissionMsg = 'You do not have permission to view invoice types. This feature requires Settings or Sales view permissions.';
        setPermissionError(permissionMsg);
        toast({
          title: 'Permission Denied',
          description: permissionMsg,
          status: 'error',
          duration: 8000,
          isClosable: true,
        });
        return;
      }
      
      // Provide more detailed error information
      let errorMessage = 'Failed to load invoice types';
      if (error.response) {
        console.error('InvoiceTypeManagement: API Error Response:', error.response);
        errorMessage = error.response.data?.message || error.response.data?.error || `HTTP ${error.response.status}: ${error.response.statusText}`;
      } else if (error.message) {
        errorMessage = error.message;
      }
      
      toast({
        title: 'Error Loading Invoice Types',
        description: errorMessage,
        status: 'error',
        duration: 10000,
        isClosable: true,
      });
      
      // Set empty array so UI still works
      setInvoiceTypes([]);
    } finally {
      setLoading(false);
    }
  };

  const handleOpenModal = (type?: InvoiceType) => {
    if (type) {
      setEditingType(type);
      reset({
        name: type.name,
        code: type.code,
        is_active: type.is_active
      });
    } else {
      setEditingType(null);
      reset({
        name: '',
        code: '',
        is_active: true
      });
    }
    setValidationError(null);
    onOpen();
  };

  const handleCloseModal = () => {
    setEditingType(null);
    setValidationError(null);
    reset();
    onClose();
  };

  const onSubmit = async (data: FormData) => {
    try {
      setIsSubmitting(true);
      setValidationError(null);

      if (editingType) {
        // Update existing invoice type
        const updateData: InvoiceTypeUpdateRequest = {
          name: data.name,
          code: data.code,
          is_active: data.is_active
        };
        await invoiceTypeService.updateInvoiceType(editingType.id, updateData);
        toast({
          title: 'Success',
          description: 'Invoice type updated successfully',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      } else {
        // Create new invoice type
        const createData: InvoiceTypeCreateRequest = {
          name: data.name,
          code: data.code,
        };
        await invoiceTypeService.createInvoiceType(createData);
        toast({
          title: 'Success',
          description: 'Invoice type created successfully',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
      }

      handleCloseModal();
      loadInvoiceTypes();
    } catch (error: any) {
      console.error('Error saving invoice type:', error);
      
      if (error.response?.status === 422 && error.response?.data?.errors) {
        // Handle validation errors
        const parsedError = ErrorHandler.parseValidationError(error);
        setValidationError(parsedError);
      } else {
        toast({
          title: 'Error',
          description: error.message || 'Failed to save invoice type',
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      }
    } finally {
      setIsSubmitting(false);
    }
  };

  const handleDelete = async (type: InvoiceType) => {
    if (window.confirm(`Are you sure you want to delete invoice type "${type.name}"?`)) {
      try {
        await invoiceTypeService.deleteInvoiceType(type.id);
        toast({
          title: 'Success',
          description: 'Invoice type deleted successfully',
          status: 'success',
          duration: 3000,
          isClosable: true,
        });
        loadInvoiceTypes();
      } catch (error: any) {
        console.error('Error deleting invoice type:', error);
        toast({
          title: 'Error',
          description: error.message || 'Failed to delete invoice type',
          status: 'error',
          duration: 5000,
          isClosable: true,
        });
      }
    }
  };

  const handlePreviewNumber = async (type: InvoiceType) => {
    try {
      const preview = await invoiceTypeService.previewInvoiceNumber(type.id);
      toast({
        title: 'Preview Invoice Number',
        description: `Next invoice number: ${preview.preview_number}`,
        status: 'info',
        duration: 5000,
        isClosable: true,
      });
    } catch (error: any) {
      console.error('Error previewing invoice number:', error);
      toast({
        title: 'Error',
        description: error.message || 'Failed to preview invoice number',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
    }
  };

  const handleOpenCounterModal = async (type: InvoiceType) => {
    setSelectedTypeForCounter(type);
    setLoadingCounter(true);
    
    try {
      // Load counter history
      const history = await invoiceTypeService.getCounterHistory(type.id);
      setCounterHistory(history);
      
      // Set default values for the form
      const currentYear = new Date().getFullYear();
      const currentCounter = history.find(h => h.year === currentYear);
      
      resetCounter({
        year: currentYear,
        counter: currentCounter?.counter || 0
      });
      
      onCounterModalOpen();
    } catch (error: any) {
      console.error('Error loading counter history:', error);
      toast({
        title: 'Error',
        description: 'Failed to load counter history',
        status: 'error',
        duration: 3000,
        isClosable: true,
      });
    } finally {
      setLoadingCounter(false);
    }
  };

  const handleCloseCounterModal = () => {
    setSelectedTypeForCounter(null);
    setCounterHistory([]);
    resetCounter();
    onCounterModalClose();
  };

  const onSubmitCounter = async (data: CounterFormData) => {
    if (!selectedTypeForCounter) return;
    
    try {
      setIsSubmitting(true);
      
      await invoiceTypeService.resetCounter(selectedTypeForCounter.id, {
        year: data.year,
        counter: data.counter
      });
      
      toast({
        title: 'Success',
        description: `Counter for ${selectedTypeForCounter.name} (${data.year}) has been set to ${data.counter}`,
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
      
      handleCloseCounterModal();
      
      // Reload counter history if modal is still open
      if (selectedTypeForCounter) {
        const history = await invoiceTypeService.getCounterHistory(selectedTypeForCounter.id);
        setCounterHistory(history);
      }
    } catch (error: any) {
      console.error('Error resetting counter:', error);
      toast({
        title: 'Error',
        description: error.message || 'Failed to reset counter',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setIsSubmitting(false);
    }
  };

  if (loading) {
    return (
      <Box p={6}>
        <Text>Loading invoice types...</Text>
      </Box>
    );
  }

  // Show permission error if user doesn't have access
  if (permissionError) {
    return (
      <Box p={6}>
        <Alert status="error" borderRadius="lg">
          <AlertIcon />
          <Box flex="1">
            <AlertTitle>Access Denied!</AlertTitle>
            <AlertDescription display="block">
              {permissionError}
              <br /><br />
              <Text fontSize="sm" color="gray.600">
                <strong>Required permissions:</strong> Settings module access with View permission
                <br />
                <strong>Your role:</strong> {user?.role || 'Unknown'}
                <br />
                <strong>Contact your administrator</strong> to request the necessary permissions.
              </Text>
            </AlertDescription>
          </Box>
          <FiLock size={24} color="red" />
        </Alert>
      </Box>
    );
  }

  return (
    <Box p={6}>
      <Flex justify="space-between" align="center" mb={6}>
        <Box>
          <Heading size="lg" color={headingColor} mb={2}>
            <FiSettings style={{ display: 'inline', marginRight: '8px' }} />
            Invoice Types
          </Heading>
          <Text color={textColor}>
            Manage invoice types and their numbering formats
            {isReadOnlyMode && (
              <Badge ml={2} colorScheme="orange" size="sm">
                Read-Only Mode
              </Badge>
            )}
          </Text>
        </Box>
        {canCreate && (
          <Button
            leftIcon={<FiPlus />}
            colorScheme="blue"
            onClick={() => handleOpenModal()}
          >
            Add Invoice Type
          </Button>
        )}
      </Flex>

      {invoiceTypes.length === 0 ? (
        <Alert status="info">
          <AlertIcon />
          <AlertDescription>
            No invoice types found. Create your first invoice type to enable custom numbering formats.
          </AlertDescription>
        </Alert>
      ) : (
        <Box bg={bg} shadow="sm" borderRadius="lg" border="1px" borderColor={borderColor} overflow="hidden">
          <Table variant="simple">
            <Thead>
              <Tr>
                <Th>Name</Th>
                <Th>Code</Th>
                <Th>Status</Th>
                <Th>Last Invoice No</Th>
                <Th>Created By</Th>
                <Th width="220px">Actions</Th>
              </Tr>
            </Thead>
            <Tbody>
              {invoiceTypes.map((type) => (
                <Tr key={type.id}>
                  <Td>
                    <Text fontWeight="medium">{type.name}</Text>
                  </Td>
                  <Td>
                    <Badge colorScheme="blue" variant="outline">
                      {type.code}
                    </Badge>
                  </Td>
                  <Td>
                    <Badge colorScheme={type.is_active ? 'green' : 'gray'}>
                      {type.is_active ? 'Active' : 'Inactive'}
                    </Badge>
                  </Td>
                  <Td>
                    <Text fontSize="sm" fontFamily="monospace" color={textColor}>
                      {new Date().getFullYear()} • Current
                    </Text>
                  </Td>
                  <Td>
                    <Text fontSize="sm" color={textColor}>
                      {type.creator ? [type.creator.first_name, type.creator.last_name].filter(Boolean).join(' ') || type.creator.username : 'Unknown'}
                    </Text>
                  </Td>
                  <Td>
                    <HStack spacing={2}>
                      {canEdit && (
                        <>
                          <IconButton
                            size="sm"
                            aria-label="Edit"
                            icon={<FiEdit />}
                            onClick={() => handleOpenModal(type)}
                          />
                          <IconButton
                            size="sm"
                            aria-label="Manage Counter"
                            icon={<FiHash />}
                            colorScheme="purple"
                            onClick={() => handleOpenCounterModal(type)}
                            title="Manage Invoice Counter"
                          />
                        </>
                      )}
                      {canDelete && (
                        <IconButton
                          size="sm"
                          aria-label="Delete"
                          icon={<FiTrash2 />}
                          colorScheme="red"
                          onClick={() => handleDelete(type)}
                        />
                      )}
                    </HStack>
                  </Td>
                </Tr>
              ))}
            </Tbody>
          </Table>
        </Box>
      )}

      {/* Add/Edit Modal */}
      <Modal isOpen={isOpen} onClose={handleCloseModal} size="md">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            {editingType ? 'Edit Invoice Type' : 'Add New Invoice Type'}
          </ModalHeader>
          <ModalCloseButton />
          
          <form onSubmit={handleSubmit(onSubmit)}>
            <ModalBody>
              <VStack spacing={4}>
                {/* Error Alert */}
                {validationError && (
                  <ErrorAlert
                    title={validationError.title}
                    message={validationError.message}
                    errors={validationError.errors}
                    type={validationError.type === 'validation' ? 'error' : 'warning'}
                    onClose={() => setValidationError(null)}
                    dismissible={true}
                    className="mb-4"
                  />
                )}

                <FormControl isRequired isInvalid={!!errors.name}>
                  <FormLabel>Name</FormLabel>
                  <Input
                    {...register('name', {
                      required: 'Name is required',
                      minLength: {
                        value: 2,
                        message: 'Name must be at least 2 characters'
                      }
                    })}
                    placeholder="e.g., Standard Invoice"
                  />
                  <FormErrorMessage>{errors.name?.message}</FormErrorMessage>
                </FormControl>

                <FormControl isRequired isInvalid={!!errors.code}>
                  <FormLabel>Code</FormLabel>
                  <Input
                    {...register('code', {
                      required: 'Code is required',
                      minLength: {
                        value: 2,
                        message: 'Code must be at least 2 characters'
                      },
                      maxLength: {
                        value: 10,
                        message: 'Code must be at most 10 characters'
                      },
                      pattern: {
                        value: /^[a-zA-Z0-9.,\-_/<> ]+$/,
                        message: 'Code can contain letters, numbers, dots, commas, hyphens, underscores, slashes, angle brackets, and spaces'
                      }
                    })}
                    placeholder="e.g., STA-C"
                    textTransform="uppercase"
                  />
                  <FormErrorMessage>{errors.code?.message}</FormErrorMessage>
                  <Text fontSize="xs" color={textColor} mt={1}>
                    Used in invoice numbering format (e.g., 0001/STA-C/I-2025)
                  </Text>
                </FormControl>

                <FormControl>
                  <HStack justify="space-between">
                    <FormLabel mb={0}>Active Status</FormLabel>
                    <Switch
                      {...register('is_active')}
                      colorScheme="green"
                      defaultChecked={true}
                    />
                  </HStack>
                  <Text fontSize="xs" color={textColor} mt={1}>
                    Only active invoice types can be used in sales forms
                  </Text>
                </FormControl>

                <Alert status="info" size="sm">
                  <AlertIcon />
                  <AlertDescription fontSize="sm">
                    <Text><strong>Invoice Numbering Format:</strong></Text>
                    <Text>NNNN/CODE/MONTH-YEAR (e.g., 0001/STA-C/I-2025)</Text>
                    <Text>• NNNN = 4-digit sequential number per type per year</Text>
                    <Text>• CODE = Your invoice type code</Text>
                    <Text>• MONTH = Roman numeral (I-XII)</Text>
                  </AlertDescription>
                </Alert>
              </VStack>
            </ModalBody>

            <ModalFooter>
              <HStack spacing={3}>
                <Button
                  leftIcon={<FiX />}
                  variant="ghost"
                  onClick={handleCloseModal}
                  isDisabled={isSubmitting}
                >
                  Cancel
                </Button>
                <Button
                  leftIcon={<FiSave />}
                  colorScheme="blue"
                  type="submit"
                  isLoading={isSubmitting}
                  loadingText={editingType ? 'Updating...' : 'Creating...'}
                >
                  {editingType ? 'Update' : 'Create'}
                </Button>
              </HStack>
            </ModalFooter>
          </form>
        </ModalContent>
      </Modal>

      {/* Counter Management Modal */}
      <Modal isOpen={isCounterModalOpen} onClose={handleCloseCounterModal} size="xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            Manage Invoice Counter: {selectedTypeForCounter?.name} ({selectedTypeForCounter?.code})
          </ModalHeader>
          <ModalCloseButton />
          
          <form onSubmit={handleSubmitCounter(onSubmitCounter)}>
            <ModalBody>
              <VStack spacing={6} align="stretch">
                {/* Info Alert */}
                <Alert status="info" borderRadius="md">
                  <AlertIcon />
                  <Box flex="1">
                    <AlertTitle fontSize="sm">Invoice Counter Management</AlertTitle>
                    <AlertDescription fontSize="xs">
                      Set the last invoice number used for this invoice type. The next invoice will use counter + 1.
                      <br />
                      <Text mt={1} fontWeight="bold">Example:</Text> If you set counter to 120, the next invoice will be 0121/CODE/MONTH-YEAR
                    </AlertDescription>
                  </Box>
                </Alert>

                {/* Counter Form */}
                <FormControl isRequired isInvalid={!!counterErrors.year}>
                  <FormLabel>Year</FormLabel>
                  <Input
                    type="number"
                    {...registerCounter('year', {
                      required: 'Year is required',
                      min: {
                        value: 2020,
                        message: 'Year must be at least 2020'
                      },
                      max: {
                        value: 2050,
                        message: 'Year must be at most 2050'
                      },
                      valueAsNumber: true
                    })}
                  />
                  <FormErrorMessage>{counterErrors.year?.message}</FormErrorMessage>
                </FormControl>

                <FormControl isRequired isInvalid={!!counterErrors.counter}>
                  <FormLabel>Last Invoice Number (Counter)</FormLabel>
                  <Input
                    type="number"
                    {...registerCounter('counter', {
                      required: 'Counter is required',
                      min: {
                        value: 0,
                        message: 'Counter must be at least 0'
                      },
                      valueAsNumber: true
                    })}
                  />
                  <FormErrorMessage>{counterErrors.counter?.message}</FormErrorMessage>
                  <Text fontSize="xs" color={textColor} mt={1}>
                    This is the LAST used number. Next invoice will be counter + 1
                  </Text>
                </FormControl>

                {/* Counter History */}
                {counterHistory.length > 0 && (
                  <Box>
                    <Text fontWeight="bold" mb={2}>Counter History:</Text>
                    <Box maxH="200px" overflowY="auto" border="1px" borderColor={borderColor} borderRadius="md" p={3}>
                      <VStack align="stretch" spacing={2}>
                        {counterHistory.map((counter) => (
                          <HStack key={counter.id} justify="space-between" p={2} bg={counterHistoryItemBg} borderRadius="md">
                            <Text fontWeight="medium">Year {counter.year}</Text>
                            <Badge colorScheme="blue" fontSize="sm" px={3} py={1}>
                              Counter: {counter.counter}
                            </Badge>
                          </HStack>
                        ))}
                      </VStack>
                    </Box>
                  </Box>
                )}

                {loadingCounter && (
                  <Text color={textColor} fontSize="sm">Loading counter data...</Text>
                )}
              </VStack>
            </ModalBody>

            <ModalFooter>
              <HStack spacing={3}>
                <Button
                  leftIcon={<FiX />}
                  variant="ghost"
                  onClick={handleCloseCounterModal}
                  isDisabled={isSubmitting}
                >
                  Cancel
                </Button>
                <Button
                  leftIcon={<FiSave />}
                  colorScheme="purple"
                  type="submit"
                  isLoading={isSubmitting}
                  loadingText="Saving..."
                >
                  Set Counter
                </Button>
              </HStack>
            </ModalFooter>
          </form>
        </ModalContent>
      </Modal>
    </Box>
  );
};

export default InvoiceTypeManagement;
