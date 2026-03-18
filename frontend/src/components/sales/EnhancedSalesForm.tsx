'use client';

import React, { useEffect, useState } from 'react';
import {
  Alert,
  AlertIcon,
  Badge,
  Box,
  FormControl,
  FormHelperText,
  FormLabel,
  HStack,
  Input,
  Select,
  Text,
  VStack,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import { formatDateWithIndonesianMonth } from '../../utils/dataFormatters';

interface SalesFormData {
  date?: string;
  payment_terms?: string;
  due_date?: string;
  [key: string]: unknown;
}

interface SalesFormProps {
  formData: SalesFormData;
  setFormData: (data: SalesFormData) => void;
}

const formatDateForInput = (dateString?: string): string => {
  if (!dateString) {
    return '';
  }

  const date = new Date(dateString);
  return Number.isNaN(date.getTime()) ? '' : date.toISOString().split('T')[0];
};

const formatDateForDisplay = (dateString?: string): string => {
  if (!dateString) {
    return '';
  }

  return formatDateWithIndonesianMonth(dateString);
};

const calculateDueDate = (invoiceDate?: string, paymentTerms?: string): string => {
  if (!invoiceDate) {
    return '';
  }

  const baseDate = new Date(invoiceDate);
  if (Number.isNaN(baseDate.getTime())) {
    return '';
  }

  const daysByTerm: Record<string, number> = {
    COD: 0,
    NET15: 15,
    NET30: 30,
    NET45: 45,
    NET60: 60,
    NET90: 90,
  };

  const dueDate = new Date(baseDate);
  dueDate.setDate(dueDate.getDate() + (daysByTerm[paymentTerms ?? ''] ?? 30));

  return dueDate.toISOString().split('T')[0];
};

const getPaymentTermsExplanation = (terms?: string, invoiceDate?: string): string => {
  if (!invoiceDate || !terms) {
    return '';
  }

  const dueDate = calculateDueDate(invoiceDate, terms);
  const invoiceDisplayDate = formatDateForDisplay(invoiceDate);
  const dueDisplayDate = formatDateForDisplay(dueDate);

  switch (terms) {
    case 'COD':
      return `Pembayaran tunai pada saat pengiriman (${invoiceDisplayDate})`;
    case 'NET15':
      return `Pembayaran dalam 15 hari dari tanggal invoice (${invoiceDisplayDate} -> ${dueDisplayDate})`;
    case 'NET30':
      return `Pembayaran dalam 30 hari dari tanggal invoice (${invoiceDisplayDate} -> ${dueDisplayDate})`;
    case 'NET45':
      return `Pembayaran dalam 45 hari dari tanggal invoice (${invoiceDisplayDate} -> ${dueDisplayDate})`;
    case 'NET60':
      return `Pembayaran dalam 60 hari dari tanggal invoice (${invoiceDisplayDate} -> ${dueDisplayDate})`;
    case 'NET90':
      return `Pembayaran dalam 90 hari dari tanggal invoice (${invoiceDisplayDate} -> ${dueDisplayDate})`;
    default:
      return `Pembayaran dalam 30 hari dari tanggal invoice (${invoiceDisplayDate} -> ${dueDisplayDate})`;
  }
};

const EnhancedSalesForm: React.FC<SalesFormProps> = ({ formData, setFormData }) => {
  const { t } = useTranslation();
  const [calculatedDueDate, setCalculatedDueDate] = useState('');
  const [paymentTermsExplanation, setPaymentTermsExplanation] = useState('');

  useEffect(() => {
    if (!formData.date || !formData.payment_terms) {
      setCalculatedDueDate('');
      setPaymentTermsExplanation('');
      return;
    }

    const nextDueDate = calculateDueDate(formData.date, formData.payment_terms);
    setCalculatedDueDate(nextDueDate);
    setPaymentTermsExplanation(getPaymentTermsExplanation(formData.payment_terms, formData.date));

    if (formData.due_date !== nextDueDate) {
      setFormData({
        ...formData,
        due_date: nextDueDate,
      });
    }
  }, [formData, setFormData]);

  const updateField = (field: 'date' | 'payment_terms', value: string) => {
    setFormData({
      ...formData,
      [field]: value,
    });
  };

  return (
    <VStack spacing={4} align="stretch">
      <Alert status="info" size="sm">
        <AlertIcon />
        <Text fontSize="sm">{t('sales.messages.dateFormatInfo')}</Text>
      </Alert>

      <HStack spacing={4}>
        <FormControl isRequired>
          <FormLabel>
            {t('sales.form.invoiceDate')} <Text as="span" color="red.500">*</Text>
          </FormLabel>
          <Input
            type="date"
            value={formatDateForInput(formData.date)}
            onChange={(event) => updateField('date', event.target.value)}
            bg="white"
          />
          <FormHelperText>
            {t('sales.tooltips.date')}
            {formData.date ? (
              <Text as="span" fontWeight="medium" color="blue.600">
                {' -> '}
                {formatDateForDisplay(formData.date)}
              </Text>
            ) : null}
          </FormHelperText>
        </FormControl>

        <FormControl>
          <FormLabel>{t('sales.form.paymentTerms')}</FormLabel>
          <Select
            value={formData.payment_terms || 'NET30'}
            onChange={(event) => updateField('payment_terms', event.target.value)}
            bg="white"
          >
            <option value="COD">{t('sales.form.paymentTermsOptions.cod')}</option>
            <option value="NET15">{t('sales.form.paymentTermsOptions.net15')}</option>
            <option value="NET30">{t('sales.form.paymentTermsOptions.net30')}</option>
            <option value="NET45">{t('sales.form.paymentTermsOptions.net45')}</option>
            <option value="NET60">{t('sales.form.paymentTermsOptions.net60')}</option>
            <option value="NET90">{t('sales.form.paymentTermsOptions.net90')}</option>
          </Select>
          {paymentTermsExplanation ? (
            <FormHelperText color="blue.600" fontWeight="medium">
              {paymentTermsExplanation}
            </FormHelperText>
          ) : null}
        </FormControl>
      </HStack>

      <FormControl>
        <FormLabel>
          {t('sales.dueDate')}
          <Badge ml={2} colorScheme="green" size="sm">
            {t('sales.form.autoCalculated')}
          </Badge>
        </FormLabel>
        <Input type="date" value={calculatedDueDate} isReadOnly bg="gray.50" cursor="not-allowed" />
        <FormHelperText color="green.600">
          {t('sales.form.dueDateAutoCalculated')}
          {calculatedDueDate ? (
            <Text as="span" fontWeight="bold">
              {' -> '}
              {formatDateForDisplay(calculatedDueDate)}
            </Text>
          ) : null}
        </FormHelperText>
      </FormControl>

      {formData.date && formData.payment_terms ? (
        <Alert status="success" size="sm">
          <AlertIcon />
          <Box>
            <Text fontWeight="medium">{t('sales.messages.confirmCalculation')}:</Text>
            <Text fontSize="sm">
              {t('sales.invoice')}: <strong>{formatDateForDisplay(formData.date)}</strong> +{' '}
              {t('sales.form.paymentTerms')}: <strong>{formData.payment_terms}</strong> ={' '}
              {t('sales.dueDate')}: <strong>{formatDateForDisplay(calculatedDueDate)}</strong>
            </Text>
          </Box>
        </Alert>
      ) : null}
    </VStack>
  );
};

export default EnhancedSalesForm;
