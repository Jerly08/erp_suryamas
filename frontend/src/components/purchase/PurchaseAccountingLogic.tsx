'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  VStack,
  HStack,
  Text,
  Badge,
  Card,
  CardBody,
  CardHeader,
  Divider,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  TableContainer,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Progress,
  Stat,
  StatLabel,
  StatNumber,
  StatHelpText,
  SimpleGrid,
  Icon,
  Flex
} from '@chakra-ui/react';
import { FiTrendingUp, FiTrendingDown, FiDollarSign, FiFileText } from 'react-icons/fi';

interface PurchaseAccountingLogicProps {
  purchaseData: any;
  products: any[];
  visible?: boolean;
}

export interface AccountingImpact {
  inventoryImpact: number;
  expenseImpact: number;
  taxDeductible: number;
  totalAmount: number;
  ppnAmount: number;
  totalCost: number;
}

export interface JournalEntry {
  account: string;
  accountCode: string;
  debit: number;
  credit: number;
  description: string;
  type: 'ASSET' | 'LIABILITY' | 'EXPENSE' | 'TAX';
}

const PurchaseAccountingLogic: React.FC<PurchaseAccountingLogicProps> = ({
  purchaseData,
  products,
  visible = true
}) => {
  const [accountingImpact, setAccountingImpact] = useState<AccountingImpact | null>(null);
  const [journalEntries, setJournalEntries] = useState<JournalEntry[]>([]);
  const [validationErrors, setValidationErrors] = useState<string[]>([]);
  const [validationWarnings, setValidationWarnings] = useState<string[]>([]);

  useEffect(() => {
    if (purchaseData && visible) {
      calculateAccountingImpact();
      generateJournalEntries();
      validateAccountingRules();
    }
  }, [purchaseData, products, visible]);

  const formatCurrency = (amount: number) => {
    return new Intl.NumberFormat('id-ID', {
      style: 'currency',
      currency: 'IDR',
      minimumFractionDigits: 0,
      maximumFractionDigits: 0
    }).format(amount);
  };

  const calculateAccountingImpact = () => {
    if (!purchaseData.items || purchaseData.items.length === 0) {
      setAccountingImpact(null);
      return;
    }

    let inventoryImpact = 0;
    let expenseImpact = 0;
    let taxDeductible = 0;
    let subtotalBeforeDiscount = 0;
    let itemDiscountAmount = 0;

    purchaseData.items.forEach((item: any) => {
      const quantity = parseFloat(item.quantity || 0);
      const unitPrice = parseFloat(item.unit_price || 0);
      const itemDiscount = parseFloat(item.discount || 0);
      const lineSubtotal = quantity * unitPrice;
      const product = products.find(p => p.id === parseInt(item.product_id));

      if (product && lineSubtotal > 0) {
        subtotalBeforeDiscount += lineSubtotal;
        itemDiscountAmount += itemDiscount;

        // Calculate net item amount after discount
        const netItemAmount = lineSubtotal - itemDiscount;

        if (product.type === 'PRODUCT' || product.category === 'INVENTORY') {
          inventoryImpact += netItemAmount;
        } else {
          expenseImpact += netItemAmount;
        }

        // Tax deductible calculation - expenses are typically deductible
        if (item.expense_account_id) {
          const accountCode = item.expense_account_id.toString();
          if (accountCode.startsWith('5') || accountCode.startsWith('6')) {
            taxDeductible += netItemAmount;
          }
        } else if (product.type === 'SERVICE') {
          taxDeductible += netItemAmount;
        }
      }
    });

    // Apply purchase-level discount to remaining amount
    const orderDiscountRate = parseFloat(purchaseData.discount || 0);
    const orderDiscountAmount = (subtotalBeforeDiscount - itemDiscountAmount) * orderDiscountRate / 100;
    const netBeforeTax = subtotalBeforeDiscount - itemDiscountAmount - orderDiscountAmount;

    // Calculate tax additions (Penambahan)
    const ppnRate = parseFloat(purchaseData.ppn_rate || 0);
    const ppnAmount = netBeforeTax * ppnRate / 100;
    const otherTaxAdditions = parseFloat(purchaseData.other_tax_additions || 0);
    const totalTaxAdditions = ppnAmount + otherTaxAdditions;

    // Calculate tax deductions (Pemotongan)
    const pph21Rate = parseFloat(purchaseData.pph21_rate || 0);
    const pph23Rate = parseFloat(purchaseData.pph23_rate || 0);
    const pph21Amount = netBeforeTax * pph21Rate / 100;
    const pph23Amount = netBeforeTax * pph23Rate / 100;
    const otherTaxDeductions = parseFloat(purchaseData.other_tax_deductions || 0);
    const totalTaxDeductions = pph21Amount + pph23Amount + otherTaxDeductions;

    // Final total amount
    const totalCost = netBeforeTax + totalTaxAdditions - totalTaxDeductions;

    // Adjust impacts based on final discount ratio
    const discountRatio = subtotalBeforeDiscount > 0 ? (netBeforeTax / subtotalBeforeDiscount) : 1;
    inventoryImpact *= discountRatio;
    expenseImpact *= discountRatio;
    taxDeductible *= discountRatio;

    setAccountingImpact({
      inventoryImpact,
      expenseImpact,
      taxDeductible,
      totalAmount: netBeforeTax,
      ppnAmount,
      totalCost
    });
  };

  const generateJournalEntries = () => {
    if (!accountingImpact) {
      setJournalEntries([]);
      return;
    }

    const entries: JournalEntry[] = [];

    // Debit entries
    if (accountingImpact.inventoryImpact > 0) {
      entries.push({
        account: 'Inventory',
        accountCode: '1300',
        debit: accountingImpact.inventoryImpact,
        credit: 0,
        description: 'Inventory purchase - increase in assets',
        type: 'ASSET'
      });
    }

    if (accountingImpact.expenseImpact > 0) {
      entries.push({
        account: 'Operating Expenses',
        accountCode: '5xxx',
        debit: accountingImpact.expenseImpact,
        credit: 0,
        description: 'Operating expenses incurred',
        type: 'EXPENSE'
      });
    }

    // PPN Input Tax (debit)
    if (accountingImpact.ppnAmount > 0) {
      entries.push({
        account: 'PPN Masukan (Input VAT)',
        accountCode: '2102-002',
        debit: accountingImpact.ppnAmount,
        credit: 0,
        description: 'VAT paid on purchases (recoverable)',
        type: 'TAX'
      });
    }

    // Credit entry - Accounts Payable
    entries.push({
      account: 'Accounts Payable',
      accountCode: '2100',
      debit: 0,
      credit: accountingImpact.totalCost,
      description: 'Amount owed to vendor',
      type: 'LIABILITY'
    });

    setJournalEntries(entries);
  };

  const validateAccountingRules = () => {
    const errors: string[] = [];
    const warnings: string[] = [];

    if (!purchaseData.items || purchaseData.items.length === 0) {
      return;
    }

    // Validate expense account assignments
    purchaseData.items.forEach((item: any, index: number) => {
      if (!item.expense_account_id) {
        errors.push(`Item ${index + 1}: Expense account is required for proper accounting`);
      }

      const product = products.find(p => p.id === parseInt(item.product_id));
      if (product) {
        const accountCode = item.expense_account_id?.toString() || '';

        if (product.type === 'SERVICE' && !accountCode.startsWith('5')) {
          warnings.push(`Item ${index + 1}: Service items typically use expense accounts (5xxx series)`);
        } else if (product.type === 'PRODUCT' && !accountCode.startsWith('1') && !accountCode.startsWith('5')) {
          warnings.push(`Item ${index + 1}: Product items should use inventory (1xxx) or expense (5xxx) accounts`);
        }
      }

      // Check for unusual quantities or prices
      const quantity = parseFloat(item.quantity || 0);
      const unitPrice = parseFloat(item.unit_price || 0);

      if (quantity > 10000) {
        warnings.push(`Item ${index + 1}: Unusually high quantity (${quantity}). Please verify.`);
      }

      if (unitPrice > 100000000) { // 100 million per unit
        warnings.push(`Item ${index + 1}: Unusually high unit price. Please verify.`);
      }
    });

    // Validate purchase amount thresholds
    if (accountingImpact && accountingImpact.totalCost > 100000000) { // 100 million threshold
      warnings.push('Purchase exceeds Rp 100,000,000. Consider splitting into multiple purchases for better cash flow management.');
    }

    if (accountingImpact && accountingImpact.totalCost > 50000000) { // 50 million threshold
      warnings.push('Purchase exceeds Rp 50,000,000. Ensure proper approval workflow and documentation.');
    }

    // Validate tax calculations
    const discount = parseFloat(purchaseData.discount || 0);
    if (discount > 50) {
      warnings.push(`Discount of ${discount}% is unusually high. Please verify vendor terms.`);
    }

    // Check for potential tax optimization
    if (accountingImpact && accountingImpact.taxDeductible < (accountingImpact.totalAmount * 0.5)) {
      warnings.push('Low tax-deductible amount. Consider reviewing expense account classifications for tax optimization.');
    }

    setValidationErrors(errors);
    setValidationWarnings(warnings);
  };

  const getJournalEntryIcon = (type: JournalEntry['type']) => {
    switch (type) {
      case 'ASSET': return FiTrendingUp;
      case 'LIABILITY': return FiTrendingDown;
      case 'EXPENSE': return FiDollarSign;
      case 'TAX': return FiFileText;
      default: return FiDollarSign;
    }
  };

  const getJournalEntryColor = (type: JournalEntry['type']) => {
    switch (type) {
      case 'ASSET': return 'green';
      case 'LIABILITY': return 'red';
      case 'EXPENSE': return 'orange';
      case 'TAX': return 'blue';
      default: return 'gray';
    }
  };

  if (!visible || !accountingImpact) {
    return null;
  }

  return (
    <VStack spacing={6} align="stretch">
      {/* Validation Alerts */}
      {validationErrors.length > 0 && (
        <Alert status="error" variant="left-accent">
          <AlertIcon />
          <Box>
            <AlertTitle>Accounting Validation Errors:</AlertTitle>
            <AlertDescription>
              <VStack align="start" spacing={1}>
                {validationErrors.map((error, index) => (
                  <Text key={index} fontSize="sm">{error}</Text>
                ))}
              </VStack>
            </AlertDescription>
          </Box>
        </Alert>
      )}

      {validationWarnings.length > 0 && (
        <Alert status="warning" variant="left-accent">
          <AlertIcon />
          <Box>
            <AlertTitle>Accounting Warnings:</AlertTitle>
            <AlertDescription>
              <VStack align="start" spacing={1}>
                {validationWarnings.map((warning, index) => (
                  <Text key={index} fontSize="sm">{warning}</Text>
                ))}
              </VStack>
            </AlertDescription>
          </Box>
        </Alert>
      )}

      {/* Accounting Impact Summary */}
      <Card>
        <CardHeader>
          <Text fontSize="lg" fontWeight="bold" color="blue.600">
            📊 Accounting Impact Summary
          </Text>
        </CardHeader>
        <CardBody>
          <SimpleGrid columns={2} spacing={4}>
            <Stat>
              <StatLabel>Inventory Impact</StatLabel>
              <StatNumber color="green.500">
                {formatCurrency(accountingImpact.inventoryImpact)}
              </StatNumber>
              <StatHelpText>Asset increase</StatHelpText>
            </Stat>

            <Stat>
              <StatLabel>Expense Impact</StatLabel>
              <StatNumber color="orange.500">
                {formatCurrency(accountingImpact.expenseImpact)}
              </StatNumber>
              <StatHelpText>Operating costs</StatHelpText>
            </Stat>

            <Stat>
              <StatLabel>Tax Deductible</StatLabel>
              <StatNumber color="blue.500">
                {formatCurrency(accountingImpact.taxDeductible)}
              </StatNumber>
              <StatHelpText>Potential tax benefit</StatHelpText>
            </Stat>

            <Stat>
              <StatLabel>Total Cost</StatLabel>
              <StatNumber color="purple.600">
                {formatCurrency(accountingImpact.totalCost)}
              </StatNumber>
              <StatHelpText>Including VAT</StatHelpText>
            </Stat>
          </SimpleGrid>

          <Box mt={4}>
            <HStack justify="space-between">
              <Text fontSize="sm" color="gray.600">Tax Deductibility Ratio:</Text>
              <Text fontSize="sm" fontWeight="medium">
                {accountingImpact.totalAmount > 0
                  ? `${((accountingImpact.taxDeductible / accountingImpact.totalAmount) * 100).toFixed(1)}%`
                  : '0%'
                }
              </Text>
            </HStack>
            <Progress
              value={accountingImpact.totalAmount > 0
                ? (accountingImpact.taxDeductible / accountingImpact.totalAmount) * 100
                : 0
              }
              colorScheme="blue"
              size="sm"
              mt={2}
            />
          </Box>
        </CardBody>
      </Card>

      {/* Journal Entries Preview */}
      <Card>
        <CardHeader>
          <Text fontSize="lg" fontWeight="bold" color="green.600">
            📋 Journal Entries Preview
          </Text>
        </CardHeader>
        <CardBody>
          <Text fontSize="sm" color="gray.600" mb={4}>
            These journal entries will be automatically created when the purchase is saved:
          </Text>

          <TableContainer>
            <Table variant="simple" size="sm">
              <Thead>
                <Tr>
                  <Th>Account</Th>
                  <Th>Description</Th>
                  <Th isNumeric>Debit</Th>
                  <Th isNumeric>Credit</Th>
                </Tr>
              </Thead>
              <Tbody>
                {journalEntries.map((entry, index) => (
                  <Tr key={index}>
                    <Td>
                      <HStack>
                        <Icon
                          as={getJournalEntryIcon(entry.type)}
                          color={`${getJournalEntryColor(entry.type)}.500`}
                          boxSize={4}
                        />
                        <VStack align="start" spacing={0}>
                          <Text fontWeight="medium" fontSize="sm">
                            {entry.accountCode} - {entry.account}
                          </Text>
                          <Badge
                            size="xs"
                            colorScheme={getJournalEntryColor(entry.type)}
                            variant="subtle"
                          >
                            {entry.type}
                          </Badge>
                        </VStack>
                      </HStack>
                    </Td>
                    <Td>
                      <Text fontSize="sm" color="gray.600">
                        {entry.description}
                      </Text>
                    </Td>
                    <Td isNumeric>
                      <Text
                        fontWeight={entry.debit > 0 ? "bold" : "normal"}
                        color={entry.debit > 0 ? "green.600" : "gray.400"}
                      >
                        {entry.debit > 0 ? formatCurrency(entry.debit) : '-'}
                      </Text>
                    </Td>
                    <Td isNumeric>
                      <Text
                        fontWeight={entry.credit > 0 ? "bold" : "normal"}
                        color={entry.credit > 0 ? "red.600" : "gray.400"}
                      >
                        {entry.credit > 0 ? formatCurrency(entry.credit) : '-'}
                      </Text>
                    </Td>
                  </Tr>
                ))}
              </Tbody>
            </Table>
          </TableContainer>

          <Divider my={4} />

          <HStack justify="space-between">
            <Text fontSize="sm" fontWeight="bold">Total:</Text>
            <HStack spacing={8}>
              <Text fontSize="sm" fontWeight="bold" color="green.600">
                Debit: {formatCurrency(journalEntries.reduce((sum, entry) => sum + entry.debit, 0))}
              </Text>
              <Text fontSize="sm" fontWeight="bold" color="red.600">
                Credit: {formatCurrency(journalEntries.reduce((sum, entry) => sum + entry.credit, 0))}
              </Text>
            </HStack>
          </HStack>
        </CardBody>
      </Card>
    </VStack>
  );
};

export default PurchaseAccountingLogic;

// Utility functions for external use
export const validatePurchaseAccountingRules = (
  purchaseData: any,
  products: any[]
): { errors: string[], warnings: string[] } => {
  const errors: string[] = [];
  const warnings: string[] = [];

  if (!purchaseData.items || purchaseData.items.length === 0) {
    return { errors, warnings };
  }

  // Validate expense account assignments
  purchaseData.items.forEach((item: any, index: number) => {
    if (!item.expense_account_id) {
      errors.push(`Item ${index + 1}: Expense account is required for proper accounting`);
    }

    const product = products.find(p => p.id === parseInt(item.product_id));
    if (product) {
      const accountCode = item.expense_account_id?.toString() || '';

      if (product.type === 'SERVICE' && !accountCode.startsWith('5')) {
        warnings.push(`Item ${index + 1}: Service items typically use expense accounts (5xxx series)`);
      }
    }
  });

  return { errors, warnings };
};

export const calculatePurchaseAccountingImpact = (
  purchaseData: any,
  products: any[]
): AccountingImpact | null => {
  if (!purchaseData.items || purchaseData.items.length === 0) {
    return null;
  }

  let inventoryImpact = 0;
  let expenseImpact = 0;
  let taxDeductible = 0;

  purchaseData.items.forEach((item: any) => {
    const itemTotal = parseFloat(item.quantity || 0) * parseFloat(item.unit_price || 0);
    const product = products.find(p => p.id === parseInt(item.product_id));

    if (product && itemTotal > 0) {
      if (product.type === 'PRODUCT') {
        inventoryImpact += itemTotal;
      } else {
        expenseImpact += itemTotal;
      }

      if (item.expense_account_id && item.expense_account_id.toString().startsWith('5')) {
        taxDeductible += itemTotal;
      }
    }
  });

  const subtotal = inventoryImpact + expenseImpact;
  const discount = parseFloat(purchaseData.discount || 0);
  const netAmount = subtotal - (subtotal * discount / 100);
  const ppnAmount = netAmount * 0.11;
  const totalCost = netAmount + ppnAmount;

  return {
    inventoryImpact,
    expenseImpact,
    taxDeductible,
    totalAmount: netAmount,
    ppnAmount,
    totalCost
  };
};
