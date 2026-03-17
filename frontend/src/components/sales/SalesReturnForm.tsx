'use client';

import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  ModalCloseButton,
  Button,
  FormControl,
  FormLabel,
  FormErrorMessage,
  Input,
  Select,
  Textarea,
  VStack,
  HStack,
  Text,
  Box,
  Divider,
  useToast,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  IconButton,
  Switch,
  Badge,
  Alert,
  AlertIcon,
  AlertDescription
} from '@chakra-ui/react';
import { useForm, useFieldArray } from 'react-hook-form';
import { FiPlus, FiTrash2, FiSave, FiX } from 'react-icons/fi';
import salesService, { Sale, SaleReturnRequest, SaleReturnItemRequest } from '../../services/salesService';
// TODO: Implement utility functions after proper setup

interface SalesReturnFormProps {
  isOpen: boolean;
  onClose: () => void;
  onSave: () => void;
  sale: Sale | null;
}

interface ReturnFormData {
  return_date: string;
  type: 'RETURN' | 'CREDIT_NOTE';
  reason: string;
  notes: string;
  return_items: Array<{
    sale_item_id: number;
    quantity: number;
    reason: string;
    selected: boolean;
  }>;
}

const SalesReturnForm: React.FC<SalesReturnFormProps> = ({
  isOpen,
  onClose,
  onSave,
  sale
}) => {
  const [loading, setLoading] = useState(false);
  const toast = useToast();

  const {
    register,
    handleSubmit,
    reset,
    setValue,
    watch,
    control,
    formState: { errors }
  } = useForm<ReturnFormData>({
    defaultValues: {
      return_date: new Date().toISOString().split('T')[0],
      type: 'RETURN',
      reason: '',
      notes: '',
      return_items: []
    }
  });

  const { fields, append, remove, update } = useFieldArray({
    control,
    name: 'return_items'
  });

  const watchReturnItems = watch('return_items');
  const watchType = watch('type');

  useEffect(() => {
    if (sale && isOpen) {
      // Initialize return items from sale items
      const returnItems = sale.sale_items?.map(item => ({
        sale_item_id: item.id || 0,
        quantity: 0,
        reason: '',
        selected: false
      })) || [];

      setValue('return_items', returnItems);
      setValue('return_date', new Date().toISOString().split('T')[0]);
      setValue('type', 'RETURN');
      setValue('reason', '');
      setValue('notes', '');
    }
  }, [sale, isOpen, setValue]);

  const handleItemSelection = (index: number, selected: boolean) => {
    const currentItem = watchReturnItems[index];
    update(index, {
      ...currentItem,
      selected,
      quantity: selected ? 1 : 0
    });
  };

  const handleQuantityChange = (index: number, quantity: number) => {
    const currentItem = watchReturnItems[index];
    const maxQuantity = sale?.sale_items?.[index]?.quantity || 0;
    
    if (quantity > maxQuantity) {
      toast({
        title: 'Invalid Quantity',
        description: `Maximum quantity available for return is ${maxQuantity}`,
        status: 'error',
        duration: 3000
      });
      return;
    }

    update(index, {
      ...currentItem,
      quantity: Math.max(0, quantity)
    });
  };

  const calculateReturnTotal = () => {
    if (!sale?.sale_items) return 0;
    
    return watchReturnItems.reduce((total, returnItem, index) => {
      if (!returnItem.selected || !returnItem.quantity) return total;
      
      const saleItem = sale.sale_items[index];
      if (!saleItem) return total;
      
      const itemTotal = saleItem.unit_price * returnItem.quantity;
      return total + itemTotal;
    }, 0);
  };

  const getSelectedItemsCount = () => {
    return watchReturnItems.filter(item => item.selected && item.quantity > 0).length;
  };

  const onSubmit = async (data: ReturnFormData) => {
    if (!sale) return;

    try {
      setLoading(true);

      const selectedItems = data.return_items.filter(item => item.selected && item.quantity > 0);
      
      if (selectedItems.length === 0) {
        toast({
          title: 'No Items Selected',
          description: 'Please select at least one item to return',
          status: 'error',
          duration: 3000
        });
        return;
      }

      const returnData: SaleReturnRequest = {
        return_date: data.return_date, // Use correct backend field name
        type: 'RETURN',
        reason: data.reason,
        notes: data.notes,
        return_items: selectedItems.map(item => ({ // Use correct backend field name
          sale_item_id: item.sale_item_id,
          quantity: item.quantity,
          reason: item.reason
        }))
      };

      await salesService.createSaleReturn(sale.id, returnData);

      toast({
        title: 'Return Created',
        description: `Return has been created successfully${data.type === 'CREDIT_NOTE' ? ' with credit note' : ''}`,
        status: 'success',
        duration: 3000
      });

      reset();
      onSave();
      onClose();
    } catch (error: any) {
      toast({
        title: 'Error Creating Return',
        description: error.response?.data?.message || 'Failed to create return',
        status: 'error',
        duration: 5000
      });
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    reset();
    onClose();
  };

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  };

  return (
    <Modal 
      isOpen={isOpen} 
      onClose={handleClose} 
      size="6xl" 
      isCentered
      closeOnOverlayClick={false}
    >
      <ModalOverlay bg="blackAlpha.700" backdropFilter="blur(4px)" />
      <ModalContent 
        maxH="95vh" 
        mx={4} 
        my={2} 
        borderRadius="xl"
        bg="white"
        shadow="2xl"
        overflow="hidden"
        display="flex"
        flexDirection="column"
      >
        <ModalHeader 
          bg="red.50" 
          borderBottomWidth={1} 
          borderColor="gray.200"
          pb={4}
          pt={6}
        >
          <HStack justify="space-between" align="center">
            <Box>
              <Text fontSize="xl" fontWeight="bold" color="red.700">
                Create Sales Return
              </Text>
              <Text color="gray.600" fontSize="sm" mt={1}>
                Process product returns and generate credit notes
              </Text>
            </Box>
            <Badge colorScheme="red" variant="solid" px={3} py={1} borderRadius="md">
              Return Process
            </Badge>
          </HStack>
        </ModalHeader>
        <ModalCloseButton />

        <form onSubmit={handleSubmit(onSubmit)} style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
          <ModalBody 
            flex="1" 
            overflowY="auto" 
            px={6} 
            py={4}
          >
            <VStack spacing={6} align="stretch">
              {/* Sale Information */}
              {sale && (
                <Box p={4} bg="gray.50" borderRadius="md">
                  <VStack align="stretch" spacing={2}>
                    <HStack justify="space-between">
                      <Text fontSize="sm" color="gray.600">Sale Code:</Text>
                      <Text fontWeight="medium">{sale.code}</Text>
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm" color="gray.600">Customer:</Text>
                      <Text fontWeight="medium">{sale.customer?.name || 'N/A'}</Text>
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm" color="gray.600">Invoice Number:</Text>
                      <Text fontWeight="medium">{sale.invoice_number || 'N/A'}</Text>
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm" color="gray.600">Total Amount:</Text>
                      <Text fontWeight="bold">{formatCurrency(sale.total_amount)}</Text>
                    </HStack>
                  </VStack>
                </Box>
              )}

              {/* Return Information */}
              <Box>
                <Text fontSize="lg" fontWeight="medium" mb={4} color="gray.700">
                  Return Information
                </Text>
                <VStack spacing={4}>
                  <HStack w="full" spacing={4}>
                    <FormControl isRequired isInvalid={!!errors.return_date}>
                      <FormLabel>Return Date</FormLabel>
                      <Input
                        type="date"
                        {...register('return_date', {
                          required: 'Return date is required'
                        })}
                      />
                      <FormErrorMessage>{errors.return_date?.message}</FormErrorMessage>
                    </FormControl>

                    <FormControl>
                      <FormLabel>Return Type</FormLabel>
                      <Select {...register('type')}>
                        <option value="RETURN">Product Return</option>
                        <option value="CREDIT_NOTE">Credit Note</option>
                      </Select>
                    </FormControl>
                  </HStack>

                  <FormControl isRequired isInvalid={!!errors.reason}>
                    <FormLabel>Return Reason</FormLabel>
                    <Textarea
                      {...register('reason', {
                        required: 'Return reason is required'
                      })}
                      placeholder="Describe the reason for this return..."
                      rows={3}
                    />
                    <FormErrorMessage>{errors.reason?.message}</FormErrorMessage>
                  </FormControl>

                  <FormControl>
                    <FormLabel>Additional Notes</FormLabel>
                    <Textarea
                      {...register('notes')}
                      placeholder="Any additional notes or comments..."
                      rows={2}
                    />
                  </FormControl>
                </VStack>
              </Box>

              <Divider />

              {/* Return Items */}
              <Box>
                <HStack justify="space-between" align="center" mb={4}>
                  <Text fontSize="lg" fontWeight="medium" color="gray.700">
                    Items to Return
                  </Text>
                  <Badge colorScheme="blue" variant="outline">
                    {getSelectedItemsCount()} items selected
                  </Badge>
                </HStack>

                {watchReturnItems.length === 0 ? (
                  <Alert status="info">
                    <AlertIcon />
                    <AlertDescription>
                      No items available for return. Please ensure the sale has items.
                    </AlertDescription>
                  </Alert>
                ) : (
                  <Box 
                    overflowX="auto" 
                    border="1px" 
                    borderColor="gray.200" 
                    borderRadius="md"
                    bg="white"
                    shadow="sm"
                  >
                    <TableContainer>
                      <Table variant="simple" size="sm">
                        <Thead>
                          <Tr>
                            <Th width="50px">Select</Th>
                            <Th>Product</Th>
                            <Th isNumeric>Sold Qty</Th>
                            <Th isNumeric>Return Qty</Th>
                            <Th isNumeric>Unit Price</Th>
                            <Th isNumeric>Return Value</Th>
                            <Th>Return Reason</Th>
                          </Tr>
                        </Thead>
                        <Tbody>
                          {watchReturnItems.map((returnItem, index) => {
                            const saleItem = sale?.sale_items?.[index];
                            if (!saleItem) return null;

                            const returnValue = returnItem.selected && returnItem.quantity 
                              ? saleItem.unit_price * returnItem.quantity 
                              : 0;

                            return (
                              <Tr key={saleItem.id}>
                                <Td>
                                  <Switch
                                    isChecked={returnItem.selected}
                                    onChange={(e) => handleItemSelection(index, e.target.checked)}
                                  />
                                </Td>
                                <Td>
                                  <Text fontWeight="medium">
                                    {saleItem.product_name || saleItem.product?.name || 'Unknown Product'}
                                  </Text>
                                  <Text fontSize="xs" color="gray.500">
                                    {saleItem.description || ''}
                                  </Text>
                                </Td>
                                <Td isNumeric>{saleItem.quantity}</Td>
                                <Td>
                                  <NumberInput
                                    size="sm"
                                    min={0}
                                    max={saleItem.quantity}
                                    value={returnItem.quantity}
                                    onChange={(_, valueNumber) => handleQuantityChange(index, valueNumber || 0)}
                                    isDisabled={!returnItem.selected}
                                    width="80px"
                                  >
                                    <NumberInputField />
                                    <NumberInputStepper>
                                      <NumberIncrementStepper />
                                      <NumberDecrementStepper />
                                    </NumberInputStepper>
                                  </NumberInput>
                                </Td>
                                <Td isNumeric>{formatCurrency(saleItem.unit_price)}</Td>
                                <Td isNumeric>
                                  <Text fontWeight={returnValue > 0 ? "bold" : "normal"}>
                                    {formatCurrency(returnValue)}
                                  </Text>
                                </Td>
                                <Td>
                                  <Input
                                    size="sm"
                                    placeholder="Item reason"
                                    value={returnItem.reason}
                                    onChange={(e) => {
                                      const currentItem = watchReturnItems[index];
                                      update(index, {
                                        ...currentItem,
                                        reason: e.target.value
                                      });
                                    }}
                                    isDisabled={!returnItem.selected}
                                    width="150px"
                                  />
                                </Td>
                              </Tr>
                            );
                          })}
                        </Tbody>
                      </Table>
                    </TableContainer>
                  </Box>
                )}

                {/* Return Summary */}
                {getSelectedItemsCount() > 0 && (
                  <Box mt={4} p={4} bg="red.50" borderRadius="md" border="1px" borderColor="red.200">
                    <HStack justify="space-between">
                      <VStack align="start" spacing={1}>
                        <Text fontSize="sm" color="red.600" fontWeight="medium">
                          Return Summary
                        </Text>
                        <Text fontSize="xs" color="red.500">
                          {getSelectedItemsCount()} items selected for return
                        </Text>
                      </VStack>
                      <VStack align="end" spacing={1}>
                        <Text fontSize="lg" fontWeight="bold" color="red.600">
                          {formatCurrency(calculateReturnTotal())}
                        </Text>
                        <Text fontSize="xs" color="red.500">
                          Total Return Value
                        </Text>
                      </VStack>
                    </HStack>
                  </Box>
                )}
              </Box>
            </VStack>
          </ModalBody>

          <ModalFooter 
            position="sticky"
            bottom={0}
            borderTopWidth={2} 
            borderColor="gray.300" 
            bg="white"
            boxShadow="0 -4px 12px rgba(0, 0, 0, 0.1)"
            px={6}
            py={4}
            mt={6}
            flexShrink={0}
            zIndex={10}
          >
            <HStack justify="space-between" spacing={4} w="full">
              {/* Left side - Return info */}
              <HStack spacing={2}>
                <Text fontSize="sm" color="gray.500">
                  {getSelectedItemsCount()} items • {formatCurrency(calculateReturnTotal())}
                </Text>
                {watchType === 'CREDIT_NOTE' && (
                  <Badge colorScheme="blue" variant="subtle" fontSize="xs">
                    Credit Note
                  </Badge>
                )}
              </HStack>
              
              {/* Right side - Action buttons */}
              <HStack spacing={3}>
                <Button
                  leftIcon={<FiX />}
                  onClick={handleClose}
                  variant="outline"
                  size="lg"
                  isDisabled={loading}
                  colorScheme="gray"
                  minW="120px"
                >
                  Cancel
                </Button>
                <Button
                  leftIcon={loading ? undefined : <FiSave />}
                  type="submit"
                  colorScheme="red"
                  size="lg"
                  isLoading={loading}
                  loadingText="Processing..."
                  minW="150px"
                  shadow="md"
                  isDisabled={getSelectedItemsCount() === 0}
                  _hover={{
                    shadow: "lg",
                    transform: "translateY(-1px)",
                  }}
                  _active={{
                    transform: "translateY(0)",
                  }}
                >
                  Create Return
                </Button>
              </HStack>
            </HStack>
          </ModalFooter>
        </form>
      </ModalContent>
    </Modal>
  );
};

export default SalesReturnForm;
