'use client';

import React, { useCallback, useState, useEffect } from 'react';
import {
  Input,
  InputGroup,
  InputLeftElement,
  Text,
  FormControl,
  FormLabel,
  FormErrorMessage,
  useColorModeValue,
} from '@chakra-ui/react';

interface CurrencyInputProps {
  value: number;
  onChange: (value: number) => void;
  placeholder?: string;
  isDisabled?: boolean;
  isInvalid?: boolean;
  size?: string;
  label?: string;
  isRequired?: boolean;
  errorMessage?: string;
  min?: number;
  max?: number;
  showLabel?: boolean;
  keepFormattedOnFocus?: boolean;
  allowDecimals?: boolean;
  decimalScale?: number;
}

const CurrencyInput: React.FC<CurrencyInputProps> = ({
  value,
  onChange,
  placeholder = "Masukkan jumlah",
  isDisabled = false,
  isInvalid = false,
  size = "md",
  label,
  isRequired = false,
  errorMessage,
  min = 0,
  max,
  showLabel = true,
  keepFormattedOnFocus = false,
  allowDecimals = true,
  decimalScale = 2,
}) => {
  const [displayValue, setDisplayValue] = useState<string>('');
  const [isFocused, setIsFocused] = useState(false);
  const isCompactSize = size === 'sm' || size === 'xs';
  const prefixWidth = isCompactSize ? '28px' : '40px';
  const inputFontSize = isCompactSize ? 'xs' : 'sm';
  const inputBg = useColorModeValue('white', 'var(--bg-secondary)');
  const inputTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const placeholderColor = useColorModeValue('gray.400', 'var(--text-secondary)');
  const borderColor = useColorModeValue(isInvalid ? 'red.500' : 'gray.300', isInvalid ? 'red.300' : 'var(--border-color)');
  const hoverBorderColor = useColorModeValue(isInvalid ? 'red.500' : 'gray.400', isInvalid ? 'red.300' : 'gray.500');
  const prefixColor = useColorModeValue('gray.500', 'var(--text-secondary)');
  const disabledBg = useColorModeValue('gray.100', 'whiteAlpha.100');
  const labelColor = useColorModeValue('gray.700', 'var(--text-primary)');
  const normalizedDecimalScale = Math.max(0, decimalScale);

  // Format number to IDR currency display
  const formatToIDR = useCallback((amount: number): string => {
    if (amount === 0) return '';
    const hasFraction = Math.abs(amount % 1) > Number.EPSILON;
    return new Intl.NumberFormat('id-ID', {
      minimumFractionDigits: allowDecimals && hasFraction ? normalizedDecimalScale : 0,
      maximumFractionDigits: allowDecimals ? normalizedDecimalScale : 0,
    }).format(amount);
  }, [allowDecimals, normalizedDecimalScale]);

  const formatDigits = (digits: string): string => {
    const cleaned = digits.replace(/[^\d]/g, '');
    if (cleaned === '') return '';
    return new Intl.NumberFormat('id-ID').format(parseInt(cleaned, 10));
  };

  const parseDecimalInput = (rawValue: string) => {
    const sanitized = rawValue.replace(/[^\d.,]/g, '');
    if (sanitized === '') {
      return { display: '', numericValue: 0 };
    }

    const hasComma = sanitized.includes(',');
    const dotCount = (sanitized.match(/\./g) || []).length;
    const lastDotIndex = sanitized.lastIndexOf('.');
    const dotDecimalCandidate =
      !hasComma &&
      dotCount === 1 &&
      lastDotIndex >= 0 &&
      sanitized.length - lastDotIndex - 1 <= normalizedDecimalScale;

    let integerDigits = sanitized.replace(/[^\d]/g, '');
    let decimalDigits = '';
    let hasDecimalSeparator = false;

    if (hasComma) {
      const lastCommaIndex = sanitized.lastIndexOf(',');
      integerDigits = sanitized.slice(0, lastCommaIndex).replace(/[^\d]/g, '');
      decimalDigits = sanitized
        .slice(lastCommaIndex + 1)
        .replace(/[^\d]/g, '')
        .slice(0, normalizedDecimalScale);
      hasDecimalSeparator = true;
    } else if (dotDecimalCandidate) {
      integerDigits = sanitized.slice(0, lastDotIndex).replace(/[^\d]/g, '');
      decimalDigits = sanitized
        .slice(lastDotIndex + 1)
        .replace(/[^\d]/g, '')
        .slice(0, normalizedDecimalScale);
      hasDecimalSeparator = true;
    }

    const integerDisplay = formatDigits(integerDigits) || '0';
    const decimalDisplay =
      hasDecimalSeparator && normalizedDecimalScale > 0 ? `,${decimalDigits}` : '';
    const numericValue = Number(
      `${integerDigits || '0'}${decimalDigits ? `.${decimalDigits}` : ''}`,
    );

    return {
      display: `${integerDisplay}${decimalDisplay}`,
      numericValue: Number.isFinite(numericValue) ? numericValue : 0,
    };
  };

  // Update display value when value prop changes
  useEffect(() => {
    if (!isFocused) {
      setDisplayValue(formatToIDR(value));
    }
  }, [value, isFocused, formatToIDR]);

  // Handle input change
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;
    
    const parsed = allowDecimals
      ? parseDecimalInput(inputValue)
      : {
          display: formatDigits(inputValue),
          numericValue:
            inputValue.replace(/[^\d]/g, '') === ''
              ? 0
              : parseInt(inputValue.replace(/[^\d]/g, ''), 10),
        };

    setDisplayValue(parsed.display);
    
    // Apply min/max constraints
    let constrainedValue = parsed.numericValue;
    if (min !== undefined && parsed.numericValue < min) {
      constrainedValue = min;
    }
    if (max !== undefined && parsed.numericValue > max) {
      constrainedValue = max;
    }
    
    onChange(constrainedValue);
  };

  // Handle focus
  const handleFocus = () => {
    setIsFocused(true);
    setDisplayValue(
      value === 0
        ? ''
        : keepFormattedOnFocus || allowDecimals
          ? formatToIDR(value)
          : value.toString(),
    );
  };

  // Handle blur
  const handleBlur = () => {
    setIsFocused(false);
    // Format back to currency display
    setDisplayValue(formatToIDR(value));
  };

  // Handle key press
  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.ctrlKey || e.metaKey) return;

    // Allow only numbers, editing/navigation keys, and decimal separators when enabled.
    const allowedKeys = [
      'Backspace',
      'Delete',
      'ArrowLeft',
      'ArrowRight',
      'ArrowUp',
      'ArrowDown',
      'Home',
      'End',
      'Tab',
    ];
    const isNumber = /^\d$/.test(e.key);
    const isDecimalSeparator = allowDecimals && [',', '.'].includes(e.key);
    
    if (!isNumber && !isDecimalSeparator && !allowedKeys.includes(e.key)) {
      e.preventDefault();
    }
  };

  const inputElement = (
    <InputGroup size={size}>
      <InputLeftElement pointerEvents="none" width={prefixWidth}>
        <Text color={prefixColor} fontSize={inputFontSize} fontWeight="medium">
          Rp
        </Text>
      </InputLeftElement>
      <Input
        value={displayValue}
        onChange={handleInputChange}
        onFocus={handleFocus}
        onBlur={handleBlur}
        onKeyDown={handleKeyPress}
        inputMode={allowDecimals ? 'decimal' : 'numeric'}
        placeholder={placeholder}
        isDisabled={isDisabled}
        isInvalid={isInvalid}
        paddingLeft={prefixWidth}
        paddingRight={isCompactSize ? '8px' : '12px'}
        textAlign="right"
        fontFamily="mono"
        fontSize={inputFontSize}
        bg={isDisabled ? disabledBg : inputBg}
        color={inputTextColor}
        borderColor={borderColor}
        _hover={{ borderColor: hoverBorderColor }}
        _focusVisible={{
          borderColor: isInvalid ? 'red.500' : 'blue.400',
          boxShadow: isInvalid ? '0 0 0 1px var(--chakra-colors-red-500)' : '0 0 0 1px var(--chakra-colors-blue-400)',
        }}
        _placeholder={{ color: placeholderColor }}
      />
    </InputGroup>
  );

  if (label && showLabel) {
    return (
      <FormControl isRequired={isRequired} isInvalid={isInvalid}>
        <FormLabel fontSize="sm" fontWeight="medium" color={labelColor}>
          {label}
        </FormLabel>
        {inputElement}
        {errorMessage && (
          <FormErrorMessage fontSize="xs">
            {errorMessage}
          </FormErrorMessage>
        )}
      </FormControl>
    );
  }

  return inputElement;
};

export default CurrencyInput;
