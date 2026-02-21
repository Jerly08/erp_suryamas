'use client';

import React, { useState, useEffect } from 'react';
import {
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalFooter,
  Button,
  FormControl,
  FormLabel,
  Input,
  Textarea,
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  Alert,
  AlertIcon,
  useToast,
  Box,
  Text,
  VStack,
  Flex,
  Badge,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  Select,
  HStack,
  Divider,
} from '@chakra-ui/react';
import { CashBank, DepositRequest } from '@/services/cashbankService';
import cashbankService from '@/services/cashbankService';
import { useAuth } from '@/contexts/AuthContext';
import { ErrorHandler } from '@/utils/errorHandler';

interface Account {
  id: number;
  code: string;
  name: string;
  type: string;
  category: string;
  is_active: boolean;
  balance: number;
}

interface DepositSourceAccounts {
  revenue: Account[];
  equity: Account[];
}

interface DepositFormImprovedProps {
  isOpen: boolean;
  onClose: () => void;
  onSuccess: () => void;
  account: CashBank | null;
}

interface DepositRequestImproved {
  account_id: number;
  date: string;
  amount: number;
  reference: string;
  notes: string;
  source_account_id?: number; // Optional revenue account ID
}

const DepositFormImproved: React.FC<DepositFormImprovedProps> = ({
  isOpen,
  onClose,
  onSuccess,
  account
}) => {
  const { token } = useAuth();
  const [formData, setFormData] = useState({
    date: new Date().toISOString().split('T')[0],
    amount: 0,
    reference: '',
    notes: '',
    source_account_id: ''
  });

  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [sourceAccounts, setSourceAccounts] = useState<DepositSourceAccounts>({ revenue: [], equity: [] });
  const [loadingAccounts, setLoadingAccounts] = useState(false);
  const toast = useToast();

  // Load deposit source accounts when form opens
  useEffect(() => {
    if (isOpen && token) {
      loadDepositSourceAccounts();
    }
  }, [isOpen, token]);

  const loadDepositSourceAccounts = async () => {
    try {
      setLoadingAccounts(true);
      const accounts = await cashbankService.getDepositSourceAccounts();
      setSourceAccounts(accounts);
    } catch (error) {
      console.error('Error loading deposit source accounts:', error);
      // Set default equity account as fallback
      setSourceAccounts({
        revenue: [], // No revenue accounts for deposits
        equity: [{ id: 0, code: '3101', name: 'Modal Pemilik (Default)', type: 'EQUITY', category: 'OWNER_EQUITY', is_active: true, balance: 0 }]
      });
    } finally {
      setLoadingAccounts(false);
    }
  };

  const handleInputChange = (field: string, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value
    }));
    setError(null);
  };

  // Enhanced error handling for deposit operations
  const handleDepositError = (error: any) => {
    const status = error?.response?.status || error?.status;

    // Handle permission error specially with simple, clear message
    if (status === 403) {
      const permissionMessage = (
        <Box>
          <Text fontSize="sm" fontWeight="medium" mb={2}>
            You don't have permission to make deposits.
          </Text>
          <Text fontSize="sm" color="gray.600" mb={3}>
            This requires "Cash & Bank" access permissions.
          </Text>
          <Box>
            <Text fontSize="sm" fontWeight="medium" mb={1}>
              What you can do:
            </Text>
            <Text fontSize="sm" color="gray.700">
              • Contact your administrator<br />
              • Request Cash & Bank permissions<br />
              • Ask your manager for help
            </Text>
          </Box>
        </Box>
      );

      setError('Access denied - insufficient permissions');
      toast({
        title: 'Access Denied',
        description: permissionMessage,
        status: 'error',
        duration: 8000,
        isClosable: true,
      });
      return;
    }

    // Handle 400 error with more specific information
    if (status === 400) {
      const validationMessage = error?.response?.data?.error || error?.response?.data?.message || 'Invalid data';
      const detailedMessage = (
        <Box>
          <Text fontSize="sm" fontWeight="medium" mb={2}>
            Request validation failed
          </Text>
          <Text fontSize="sm" color="gray.600" mb={2}>
            {validationMessage}
          </Text>
          <Text fontSize="xs" color="gray.500">
            Please check your input and try again.
          </Text>
        </Box>
      );

      setError(`Validation error: ${validationMessage}`);
      toast({
        title: 'Invalid Request',
        description: detailedMessage,
        status: 'warning',
        duration: 8000,
        isClosable: true,
      });
      return;
    }

    // Use existing error handler for other errors
    const errorMessage = ErrorHandler.handleAPIError(error, toast, {
      operation: 'process deposit',
      fallbackMessage: 'Failed to process deposit. Please try again.',
      duration: 6000
    });

    setError(errorMessage);
  };

  const handleSubmit = async () => {
    try {
      setLoading(true);
      setError(null);

      // Basic validation
      if (!account) {
        throw new Error('No account selected');
      }

      if (formData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      // Validate and prepare request data
      const requestData: DepositRequest = {
        account_id: Number(account.id),
        date: formData.date, // Should be in YYYY-MM-DD format
        amount: Number(formData.amount),
        reference: formData.reference || '',
        notes: formData.notes || '',
      };

      // Validate required fields
      if (!requestData.account_id || requestData.account_id <= 0) {
        throw new Error('Invalid account selected');
      }

      if (!requestData.date) {
        throw new Error('Transaction date is required');
      }

      if (!requestData.amount || requestData.amount <= 0) {
        throw new Error('Amount must be greater than zero');
      }

      // Validate date format (YYYY-MM-DD)
      const dateRegex = /^\d{4}-\d{2}-\d{2}$/;
      if (!dateRegex.test(requestData.date)) {
        throw new Error('Invalid date format. Please use YYYY-MM-DD format.');
      }

      // Add source account ID if selected (not default)
      if (formData.source_account_id && formData.source_account_id !== '' && parseInt(formData.source_account_id) > 0) {
        requestData.source_account_id = parseInt(formData.source_account_id);
      }

      // Debug logging
      console.log('🔍 Deposit request data:', requestData);
      console.log('🔍 Account info:', account);
      console.log('🔍 Form data:', formData);

      // Use cashbankService instead of direct fetch
      await cashbankService.processDeposit(requestData);

      // If we get here, it was successful
      const allAccounts = [...sourceAccounts.equity]; // Only equity accounts
      const selectedAccount = allAccounts.find(acc => acc.id === parseInt(formData.source_account_id || '0'));
      const sourceAccountName = selectedAccount ? selectedAccount.name : 'Modal Disetor (Default)';

      toast({
        title: 'Deposit Successful! 💰',
        description: (
          <Box>
            <Text fontSize="sm" fontWeight="bold">Rp {formData.amount.toLocaleString('id-ID')} deposited to {account.name}</Text>
            <Text fontSize="xs" color="gray.200" mt={1}>
              ✅ Debit: {account.name} (+Rp {formData.amount.toLocaleString('id-ID')})
            </Text>
            <Text fontSize="xs" color="gray.200">
              ✅ Credit: {sourceAccountName} (+Rp {formData.amount.toLocaleString('id-ID')})
            </Text>
            <Text fontSize="xs" color="green.200" mt={1} fontWeight="bold">
              📊 Double-entry balanced automatically!
            </Text>
          </Box>
        ),
        status: 'success',
        duration: 5000,
        isClosable: true,
      });

      onSuccess();
      onClose();
    } catch (err: any) {
      console.error('Error processing deposit:', err);
      handleDepositError(err);
    } finally {
      setLoading(false);
    }
  };

  const handleClose = () => {
    setError(null);
    setFormData({
      date: new Date().toISOString().split('T')[0],
      amount: 0,
      reference: '',
      notes: '',
      source_account_id: ''
    });
    onClose();
  };

  if (!account) return null;

  const newBalance = account.balance + formData.amount;
  const allAccounts = [...sourceAccounts.equity]; // Only equity accounts
  const selectedAccount = allAccounts.find(acc => acc.id === parseInt(formData.source_account_id || '0'));
  const sourceAccountName = selectedAccount ? selectedAccount.name : 'Modal Disetor (Default)';

  return (
    <Modal isOpen={isOpen} onClose={handleClose} size="2xl" scrollBehavior="inside">
      <ModalOverlay bg="blackAlpha.600" />
      <ModalContent>
        <ModalHeader>
          <Flex alignItems="center" gap={3}>
            <Text fontSize="lg">💰</Text>
            <Box>
              <Text fontSize="lg" fontWeight="bold">
                Make Deposit
              </Text>
              <Text fontSize="sm" color="gray.500" fontFamily="mono">
                {account.name} ({account.code})
              </Text>
            </Box>
            <Badge colorScheme="green" variant="solid" fontSize="xs">
              AUTOMATIC MODE
            </Badge>
          </Flex>
        </ModalHeader>

        <ModalBody>
          {error && (
            <Alert
              status={error.includes('Access denied') || error.includes('permission') ? 'error' : 'error'}
              mb={4}
              variant="subtle"
              borderRadius="md"
            >
              <AlertIcon />
              <Box>
                <Text fontSize="sm" fontWeight="medium">
                  {error}
                </Text>
                {(error.includes('Access denied') || error.includes('permission')) && (
                  <Text fontSize="xs" color="gray.600" mt={1}>
                    Check the notification above for more details.
                  </Text>
                )}
              </Box>
            </Alert>
          )}

          <VStack spacing={6} align="stretch">
            {/* Account Information */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                💼 Account Information
              </Text>
              <Box bg="gray.50" p={4} borderRadius="md">
                <Flex justify="space-between" align="center" mb={2}>
                  <Box>
                    <Text fontSize="sm" color="gray.600" mb={1}>Account Name</Text>
                    <Text fontWeight="medium">{account.name}</Text>
                  </Box>
                  <Badge colorScheme={account.type === 'CASH' ? 'green' : 'blue'}>
                    {account.type}
                  </Badge>
                </Flex>

                <Stat>
                  <StatLabel>Current Balance</StatLabel>
                  <StatNumber
                    color={account.balance < 0 ? 'red.500' : 'green.600'}
                    fontFamily="mono"
                  >
                    Rp {Math.abs(account.balance).toLocaleString('id-ID')}
                    {account.balance < 0 && ' (Dr)'}
                  </StatNumber>
                  <StatHelpText>
                    {account.balance < 0 ? '⚠️ Overdraft' :
                      account.balance > 0 ? '✅ Available' : '➖ Zero Balance'}
                  </StatHelpText>
                </Stat>
              </Box>
            </Box>

            {/* Transaction Form */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                📝 Transaction Details
              </Text>

              <VStack spacing={4} align="stretch">
                <FormControl isRequired>
                  <FormLabel>Transaction Date</FormLabel>
                  <Input
                    type="date"
                    value={formData.date}
                    onChange={(e) => handleInputChange('date', e.target.value)}
                  />
                </FormControl>

                <FormControl isRequired>
                  <FormLabel>Amount (Rupiah)</FormLabel>
                  <Box w="100%">
                    <NumberInput
                      value={formData.amount}
                      onChange={(_, value) => handleInputChange('amount', value || 0)}
                      min={0}
                      precision={0} // No decimals for Rupiah
                      format={(val) => `Rp ${val.toLocaleString('id-ID')}`}
                      parse={(val) => {
                        // Handle Indonesian format: Rp 1.000.000 (dots as thousand separators)
                        return val.replace(/^Rp\s?/, '').replace(/\./g, '').replace(/,/g, '');
                      }}
                      w="100%"
                    >
                      <NumberInputField
                        placeholder="Rp 0"
                        fontFamily="mono"
                        fontSize="md"
                        textAlign="left"
                        pr="12" // Add padding for stepper
                      />
                      <NumberInputStepper>
                        <NumberIncrementStepper />
                        <NumberDecrementStepper />
                      </NumberInputStepper>
                    </NumberInput>
                  </Box>
                  <Text fontSize="xs" color="gray.500" mt={1}>
                    💡 Enter amount without decimals (e.g., 1000000 for Rp 1.000.000)
                  </Text>
                </FormControl>

                <FormControl>
                  <FormLabel>Reference</FormLabel>
                  <Input
                    value={formData.reference}
                    onChange={(e) => handleInputChange('reference', e.target.value)}
                    placeholder="e.g., Receipt #123, Bank slip, etc."
                  />
                </FormControl>

                <FormControl>
                  <FormLabel>Notes</FormLabel>
                  <Textarea
                    value={formData.notes}
                    onChange={(e) => handleInputChange('notes', e.target.value)}
                    placeholder="Optional transaction notes"
                    rows={3}
                  />
                </FormControl>
              </VStack>
            </Box>

            {/* Deposit Source Account Selection */}
            <Box>
              <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                💰 Capital/Equity Account Selection
              </Text>
              <Text fontSize="sm" color="gray.600" mb={3}>
                Select the equity account to be credited. Leave blank to use default "Modal Disetor" (Owner's Equity) account for proper balance sheet treatment.
              </Text>

              <FormControl>
                <FormLabel>💳 Credit Account (Equity Source)</FormLabel>
                <Select
                  value={formData.source_account_id}
                  onChange={(e) => handleInputChange('source_account_id', e.target.value)}
                  placeholder="Use default 'Modal Disetor' (Owner's Equity) account"
                  isDisabled={loadingAccounts}
                >
                  {/* EQUITY SECTION ONLY */}
                  {sourceAccounts.equity.length > 0 && (
                    sourceAccounts.equity.map((account) => (
                      <option key={`equity-${account.id}`} value={account.id}>
                        {account.code} - {account.name}
                        {account.balance !== 0 && ` (Balance: ${account.balance.toLocaleString('id-ID')})`}
                      </option>
                    ))
                  )}
                </Select>
                {loadingAccounts && (
                  <Text fontSize="xs" color="gray.500" mt={1}>
                    Loading equity accounts...
                  </Text>
                )}

                {/* Updated Help Text */}
                <Text fontSize="xs" color="gray.500" mt={2}>
                  💡 <strong>Proper Accounting:</strong> Deposits are recorded as capital contributions to maintain balanced balance sheets.
                  Only equity accounts are available to ensure correct accounting treatment.
                </Text>
              </FormControl>
            </Box>

            {/* Double-Entry Preview */}
            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  📈 Double-Entry Preview
                </Text>
                <Box bg="blue.50" p={4} borderRadius="md" border="1px solid" borderColor="blue.200">
                  <VStack spacing={3} align="stretch">
                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold" color="blue.800">
                        Journal Entry (Auto-Generated):
                      </Text>
                      <Badge colorScheme="blue" variant="solid" fontSize="xs">
                        BALANCED
                      </Badge>
                    </HStack>

                    <Divider />

                    {/* Debit Entry */}
                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="green" variant="outline" fontSize="xs">DR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium">{account.name}</Text>
                          <Text fontSize="xs" color="gray.600" fontFamily="mono">{account.code}</Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="green.600">
                        +Rp {formData.amount.toLocaleString('id-ID')}
                      </Text>
                    </HStack>

                    {/* Credit Entry */}
                    <HStack justify="space-between" align="center">
                      <HStack spacing={2}>
                        <Badge colorScheme="orange" variant="outline" fontSize="xs">CR</Badge>
                        <Box>
                          <Text fontSize="sm" fontWeight="medium">{sourceAccountName}</Text>
                          <Text fontSize="xs" color="gray.600" fontFamily="mono">
                            {selectedAccount?.code || '3101'}
                          </Text>
                        </Box>
                      </HStack>
                      <Text fontSize="sm" fontWeight="bold" fontFamily="mono" color="orange.600">
                        +Rp {formData.amount.toLocaleString('id-ID')}
                      </Text>
                    </HStack>

                    <Divider />

                    <HStack justify="space-between">
                      <Text fontSize="sm" fontWeight="bold">Total Balance:</Text>
                      <Text fontSize="sm" fontWeight="bold" color="green.600">
                        DR Rp {formData.amount.toLocaleString('id-ID')} = CR Rp {formData.amount.toLocaleString('id-ID')} ✅
                      </Text>
                    </HStack>
                  </VStack>
                </Box>
              </Box>
            )}

            {/* Balance Preview */}
            {formData.amount > 0 && (
              <Box>
                <Text fontSize="md" fontWeight="semibold" color="gray.700" mb={3}>
                  💡 Balance Preview
                </Text>
                <Alert status={newBalance < 0 ? 'warning' : 'success'} borderRadius="md">
                  <AlertIcon />
                  <Box>
                    <Text fontSize="sm" fontWeight="medium">
                      New balance after deposit:
                    </Text>
                    <Text fontSize="lg" fontWeight="bold" fontFamily="mono" mt={1}>
                      Rp {Math.abs(newBalance).toLocaleString('id-ID')}
                      {newBalance < 0 && ' (Dr)'}
                    </Text>
                    {newBalance < 0 && (
                      <Text fontSize="xs" color="orange.600" mt={1}>
                        ⚠️ This will result in an overdraft
                      </Text>
                    )}
                  </Box>
                </Alert>
              </Box>
            )}
          </VStack>
        </ModalBody>

        <ModalFooter>
          <Button variant="ghost" mr={3} onClick={handleClose} isDisabled={loading}>
            Cancel
          </Button>
          <Button
            colorScheme="green"
            onClick={handleSubmit}
            isLoading={loading}
            loadingText="Processing deposit..."
            isDisabled={formData.amount <= 0}
          >
            Process Deposit
          </Button>
        </ModalFooter>
      </ModalContent>
    </Modal>
  );
};

export default DepositFormImproved;
