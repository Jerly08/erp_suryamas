'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  VStack,
  HStack,
  FormControl,
  FormLabel,
  Input,
  Select,
  Textarea,
  Button,
  Card,
  CardHeader,
  CardBody,
  Heading,
  useToast,
  Alert,
  AlertIcon,
  Divider,
  Text,
  Switch,
  Icon,
  Badge,
  Flex,
  Spacer,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiSave, FiEye, FiRefreshCw } from 'react-icons/fi';
import CurrencyInput from '@/components/common/CurrencyInput';
import paymentService from '@/services/paymentService';
import JournalEntryPreview from './JournalEntryPreview';
import type {
  PaymentWithJournalRequest,
  CreatePaymentWithJournalRequest,
  PaymentJournalResult,
} from '@/services/paymentService';

interface PaymentWithJournalFormProps {
  onSubmit?: (paymentId: string) => void;
  onCancel?: () => void;
  initialData?: Partial<CreatePaymentWithJournalRequest>;
}

interface FormData {
  payment_type: 'RECEIVE' | 'SEND';
  amount: number;
  currency: string;
  payment_method: string;
  description: string;
  reference_id?: string;
  customer_id?: string;
  vendor_id?: string;
  invoice_id?: string;
  metadata?: Record<string, any>;
  journal_options: {
    auto_post: boolean;
    generate_reference: boolean;
    validate_balance: boolean;
    update_account_balances: boolean;
  };
}

const PaymentWithJournalForm: React.FC<PaymentWithJournalFormProps> = ({
  onSubmit,
  onCancel,
  initialData
}) => {
  const [formData, setFormData] = useState<FormData>({
    payment_type: 'RECEIVE',
    amount: 0,
    currency: 'IDR',
    payment_method: 'BANK_TRANSFER',
    description: '',
    reference_id: '',
    customer_id: '',
    vendor_id: '',
    invoice_id: '',
    metadata: {},
    journal_options: {
      auto_post: true,
      generate_reference: true,
      validate_balance: true,
      update_account_balances: true,
    },
    ...initialData
  });

  const [journalPreview, setJournalPreview] = useState<PaymentJournalResult | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [isPreviewLoading, setIsPreviewLoading] = useState(false);
  const [isPreviewExpanded, setIsPreviewExpanded] = useState(true);
  const [autoPreview, setAutoPreview] = useState(true);

  const toast = useToast();
  const bgColor = useColorModeValue('white', 'gray.800');

  // Auto-preview when form data changes (with debounce)
  useEffect(() => {
    if (autoPreview && formData.amount > 0) {
      const timeoutId = setTimeout(() => {
        handlePreviewJournal();
      }, 1000);
      return () => clearTimeout(timeoutId);
    }
  }, [formData, autoPreview]);

  const handleInputChange = (field: string, value: any) => {
    if (field.startsWith('journal_options.')) {
      const optionKey = field.replace('journal_options.', '');
      setFormData(prev => ({
        ...prev,
        journal_options: {
          ...prev.journal_options,
          [optionKey]: value
        }
      }));
    } else {
      setFormData(prev => ({
        ...prev,
        [field]: value
      }));
    }
  };

  const handlePreviewJournal = async () => {
    if (formData.amount <= 0) {
      setJournalPreview(null);
      return;
    }

    setIsPreviewLoading(true);
    try {
      // Convert form data to backend-compatible format
      const contactId = formData.customer_id ? parseInt(formData.customer_id) : 
                       formData.vendor_id ? parseInt(formData.vendor_id) : 0;
      
      const previewRequest: PaymentWithJournalRequest = {
        contact_id: contactId,
        cash_bank_id: 1, // Default cash/bank account - should be configurable
        amount: formData.amount,
        date: new Date().toISOString(),
        method: formData.payment_method,
        reference: formData.reference_id || '',
        notes: formData.description,
        auto_create_journal: formData.journal_options.auto_post,
        preview_journal: true,
        target_invoice_id: formData.invoice_id ? parseInt(formData.invoice_id) : undefined,
      };

      const result = await paymentService.previewPaymentJournal(previewRequest);
      setJournalPreview(result);
    } catch (error: any) {
      toast({
        title: 'Preview Error',
        description: error.message || 'Failed to generate journal preview',
        status: 'error',
        duration: 5000,
        isClosable: true
      });
      setJournalPreview(null);
    } finally {
      setIsPreviewLoading(false);
    }
  };

  const handleSubmit = async () => {
    if (formData.amount <= 0) {
      toast({
        title: 'Invalid Amount',
        description: 'Please enter a valid amount greater than 0',
        status: 'error',
        duration: 3000,
        isClosable: true
      });
      return;
    }

    setIsLoading(true);
    try {
      // Convert form data to backend-compatible format
      const contactId = formData.customer_id ? parseInt(formData.customer_id) : 
                       formData.vendor_id ? parseInt(formData.vendor_id) : 0;
      
      const request: PaymentWithJournalRequest = {
        contact_id: contactId,
        cash_bank_id: 1, // Default cash/bank account - should be configurable
        amount: formData.amount,
        date: new Date().toISOString(),
        method: formData.payment_method,
        reference: formData.reference_id || '',
        notes: formData.description,
        auto_create_journal: formData.journal_options.auto_post,
        target_invoice_id: formData.invoice_id ? parseInt(formData.invoice_id) : undefined,
      };

      const result = await paymentService.createPaymentWithJournal(request);
      
      toast({
        title: 'Payment Created',
        description: `Payment ${result.payment.code} created successfully with journal entry`,
        status: 'success',
        duration: 5000,
        isClosable: true
      });

      if (onSubmit) {
        onSubmit(result.payment.id.toString());
      }
    } catch (error: any) {
      toast({
        title: 'Creation Failed',
        description: error.message || 'Failed to create payment',
        status: 'error',
        duration: 5000,
        isClosable: true
      });
    } finally {
      setIsLoading(false);
    }
  };

  return (
    <Box maxW="6xl" mx="auto" p={6}>
      <VStack spacing={6} align="stretch">
        {/* Form Header */}
        <Card bg={bgColor}>
          <CardHeader>
            <Flex align="center">
              <Heading size="md">Create Payment with Journal Entry</Heading>
              <Spacer />
              <HStack>
                <Text fontSize="sm">Auto Preview</Text>
                <Switch
                  isChecked={autoPreview}
                  onChange={(e) => setAutoPreview(e.target.checked)}
                  size="sm"
                />
              </HStack>
            </Flex>
          </CardHeader>
        </Card>

        <HStack spacing={6} align="start">
          {/* Payment Form */}
          <Card bg={bgColor} flex="1">
            <CardHeader>
              <Heading size="sm">Payment Details</Heading>
            </CardHeader>
            <CardBody>
              <VStack spacing={4}>
                {/* Payment Type and Amount */}
                <HStack spacing={4} w="full">
                  <FormControl>
                    <FormLabel>Payment Type</FormLabel>
                    <Select
                      value={formData.payment_type}
                      onChange={(e) => handleInputChange('payment_type', e.target.value as 'RECEIVE' | 'SEND')}
                    >
                      <option value="RECEIVE">Receive Payment</option>
                      <option value="SEND">Send Payment</option>
                    </Select>
                  </FormControl>
                  <FormControl>
                    <FormLabel>Amount</FormLabel>
                    <CurrencyInput
                      value={formData.amount}
                      onChange={(value) => handleInputChange('amount', value || 0)}
                      min={0}
                      showLabel={false}
                    />
                  </FormControl>
                </HStack>

                {/* Currency and Payment Method */}
                <HStack spacing={4} w="full">
                  <FormControl>
                    <FormLabel>Currency</FormLabel>
                    <Select
                      value={formData.currency}
                      onChange={(e) => handleInputChange('currency', e.target.value)}
                    >
                      <option value="IDR">Indonesian Rupiah (IDR)</option>
                      <option value="USD">US Dollar (USD)</option>
                      <option value="EUR">Euro (EUR)</option>
                    </Select>
                  </FormControl>
                  <FormControl>
                    <FormLabel>Payment Method</FormLabel>
                    <Select
                      value={formData.payment_method}
                      onChange={(e) => handleInputChange('payment_method', e.target.value)}
                    >
                      <option value="BANK_TRANSFER">Bank Transfer</option>
                      <option value="CASH">Cash</option>
                      <option value="CREDIT_CARD">Credit Card</option>
                      <option value="DEBIT_CARD">Debit Card</option>
                      <option value="CHECK">Check</option>
                      <option value="DIGITAL_WALLET">Digital Wallet</option>
                    </Select>
                  </FormControl>
                </HStack>

                {/* Related Entities */}
                <HStack spacing={4} w="full">
                  <FormControl>
                    <FormLabel>Customer ID</FormLabel>
                    <Input
                      value={formData.customer_id || ''}
                      onChange={(e) => handleInputChange('customer_id', e.target.value)}
                      placeholder="Enter customer ID (for receives)"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel>Vendor ID</FormLabel>
                    <Input
                      value={formData.vendor_id || ''}
                      onChange={(e) => handleInputChange('vendor_id', e.target.value)}
                      placeholder="Enter vendor ID (for sends)"
                    />
                  </FormControl>
                </HStack>

                {/* Reference and Invoice */}
                <HStack spacing={4} w="full">
                  <FormControl>
                    <FormLabel>Reference ID</FormLabel>
                    <Input
                      value={formData.reference_id || ''}
                      onChange={(e) => handleInputChange('reference_id', e.target.value)}
                      placeholder="External reference ID"
                    />
                  </FormControl>
                  <FormControl>
                    <FormLabel>Invoice ID</FormLabel>
                    <Input
                      value={formData.invoice_id || ''}
                      onChange={(e) => handleInputChange('invoice_id', e.target.value)}
                      placeholder="Related invoice ID"
                    />
                  </FormControl>
                </HStack>

                {/* Description */}
                <FormControl>
                  <FormLabel>Description</FormLabel>
                  <Textarea
                    value={formData.description}
                    onChange={(e) => handleInputChange('description', e.target.value)}
                    placeholder="Payment description"
                    rows={3}
                  />
                </FormControl>

                <Divider />

                {/* Journal Options */}
                <Box w="full">
                  <Text fontWeight="medium" mb={3}>Journal Entry Options</Text>
                  <VStack spacing={3} align="stretch">
                    <HStack justify="space-between">
                      <Text fontSize="sm">Auto-post journal entry</Text>
                      <Switch
                        isChecked={formData.journal_options.auto_post}
                        onChange={(e) => handleInputChange('journal_options.auto_post', e.target.checked)}
                      />
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm">Generate journal reference</Text>
                      <Switch
                        isChecked={formData.journal_options.generate_reference}
                        onChange={(e) => handleInputChange('journal_options.generate_reference', e.target.checked)}
                      />
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm">Validate journal balance</Text>
                      <Switch
                        isChecked={formData.journal_options.validate_balance}
                        onChange={(e) => handleInputChange('journal_options.validate_balance', e.target.checked)}
                      />
                    </HStack>
                    <HStack justify="space-between">
                      <Text fontSize="sm">Update account balances</Text>
                      <Switch
                        isChecked={formData.journal_options.update_account_balances}
                        onChange={(e) => handleInputChange('journal_options.update_account_balances', e.target.checked)}
                      />
                    </HStack>
                  </VStack>
                </Box>

                {/* Action Buttons */}
                <HStack w="full" spacing={4}>
                  <Button
                    leftIcon={<Icon as={FiEye} />}
                    variant="outline"
                    onClick={handlePreviewJournal}
                    isLoading={isPreviewLoading}
                    loadingText="Previewing..."
                    size="sm"
                  >
                    Preview Journal
                  </Button>
                  <Spacer />
                  {onCancel && (
                    <Button variant="ghost" onClick={onCancel}>
                      Cancel
                    </Button>
                  )}
                  <Button
                    leftIcon={<Icon as={FiSave} />}
                    colorScheme="blue"
                    onClick={handleSubmit}
                    isLoading={isLoading}
                    loadingText="Creating..."
                    disabled={formData.amount <= 0}
                  >
                    Create Payment
                  </Button>
                </HStack>
              </VStack>
            </CardBody>
          </Card>

          {/* Journal Preview */}
          <Box flex="1" minW="400px">
            {journalPreview ? (
              <JournalEntryPreview
                journalResult={journalPreview}
                title="Journal Entry Preview"
                isPreview={true}
                showAccountUpdates={true}
                isExpanded={isPreviewExpanded}
                onToggleExpand={() => setIsPreviewExpanded(!isPreviewExpanded)}
              />
            ) : (
              <Card bg={bgColor}>
                <CardBody>
                  <VStack spacing={4} align="center" py={8}>
                    <Icon as={FiRefreshCw} boxSize={8} color="gray.400" />
                    <Text color="gray.500" textAlign="center">
                      {formData.amount > 0 
                        ? "Click 'Preview Journal' to see the journal entry that will be created"
                        : "Enter payment amount to preview journal entry"
                      }
                    </Text>
                    {formData.amount > 0 && (
                      <Button
                        size="sm"
                        variant="outline"
                        leftIcon={<Icon as={FiEye} />}
                        onClick={handlePreviewJournal}
                        isLoading={isPreviewLoading}
                      >
                        Generate Preview
                      </Button>
                    )}
                  </VStack>
                </CardBody>
              </Card>
            )}
          </Box>
        </HStack>
      </VStack>
    </Box>
  );
};

export default PaymentWithJournalForm;
