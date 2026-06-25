'use client';

import React from 'react';
import {
  NumberInput,
  NumberInputField,
  NumberInputStepper,
  NumberIncrementStepper,
  NumberDecrementStepper,
  FormControl,
  FormLabel,
  FormHelperText,
  FormErrorMessage,
} from '@chakra-ui/react';

interface RupiahInputProps {
  value: number;
  onChange: (value: number) => void;
  label?: string;
  placeholder?: string;
  helperText?: string;
  errorMessage?: string;
  isRequired?: boolean;
  isInvalid?: boolean;
  isDisabled?: boolean;
  min?: number;
  max?: number;
  size?: 'xs' | 'sm' | 'md' | 'lg';
  allowDecimals?: boolean;
}

const RupiahInput: React.FC<RupiahInputProps> = ({
  value,
  onChange,
  label,
  placeholder = "Rp 0",
  helperText,
  errorMessage,
  isRequired = false,
  isInvalid = false,
  isDisabled = false,
  min = 0,
  max,
  size = 'md',
  allowDecimals = true,
}) => {
  // Format number as Rupiah
  const formatRupiah = (val: number | string): string => {
    if (!val || val === 0) return '';
    const numValue = typeof val === 'string' ? parseFloat(val) : val;
    if (isNaN(numValue)) return '';
    const hasFraction = Math.abs(numValue % 1) > Number.EPSILON;
    return `Rp ${numValue.toLocaleString('id-ID', {
      minimumFractionDigits: allowDecimals && hasFraction ? 2 : 0,
      maximumFractionDigits: allowDecimals ? 2 : 0,
    })}`;
  };

  // Parse Rupiah string to number
  const parseRupiah = (val: string): string => {
    const sanitized = val.replace(/^Rp\s?/, '').replace(/[^\d.,]/g, '');
    if (!allowDecimals) {
      return sanitized.replace(/[^\d]/g, '');
    }

    if (sanitized.includes(',')) {
      const lastCommaIndex = sanitized.lastIndexOf(',');
      const integerPart = sanitized.slice(0, lastCommaIndex).replace(/[^\d]/g, '');
      const decimalPart = sanitized.slice(lastCommaIndex + 1).replace(/[^\d]/g, '').slice(0, 2);
      return `${integerPart || '0'}${decimalPart ? `.${decimalPart}` : ''}`;
    }

    const dotCount = (sanitized.match(/\./g) || []).length;
    const lastDotIndex = sanitized.lastIndexOf('.');
    if (dotCount === 1 && lastDotIndex >= 0 && sanitized.length - lastDotIndex - 1 <= 2) {
      const integerPart = sanitized.slice(0, lastDotIndex).replace(/[^\d]/g, '');
      const decimalPart = sanitized.slice(lastDotIndex + 1).replace(/[^\d]/g, '').slice(0, 2);
      return `${integerPart || '0'}${decimalPart ? `.${decimalPart}` : ''}`;
    }

    return sanitized.replace(/[^\d]/g, '');
  };

  const handleChange = (valueString: string, valueNumber: number) => {
    if (isNaN(valueNumber)) {
      onChange(0);
    } else {
      onChange(valueNumber);
    }
  };

  const inputComponent = (
    <NumberInput
      value={value}
      onChange={handleChange}
      min={min}
      max={max}
      precision={allowDecimals ? 2 : 0}
      format={formatRupiah}
      parse={parseRupiah}
      size={size}
      isDisabled={isDisabled}
    >
      <NumberInputField 
        placeholder={placeholder}
        fontFamily="mono"
      />
      <NumberInputStepper>
        <NumberIncrementStepper />
        <NumberDecrementStepper />
      </NumberInputStepper>
    </NumberInput>
  );

  // If no label, return just the input
  if (!label) {
    return inputComponent;
  }

  // Return with FormControl wrapper
  return (
    <FormControl isRequired={isRequired} isInvalid={isInvalid}>
      <FormLabel>{label}</FormLabel>
      {inputComponent}
      {helperText && <FormHelperText>{helperText}</FormHelperText>}
      {errorMessage && <FormErrorMessage>{errorMessage}</FormErrorMessage>}
    </FormControl>
  );
};

export default RupiahInput;
