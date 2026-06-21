'use client';

import React, { useState, useEffect } from 'react';
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

  // Format number to IDR currency display
  const formatToIDR = (amount: number): string => {
    if (amount === 0) return '';
    return new Intl.NumberFormat('id-ID').format(amount);
  };

  // Update display value when value prop changes
  useEffect(() => {
    if (!isFocused) {
      setDisplayValue(formatToIDR(value));
    }
  }, [value, isFocused]);

  // Handle input change
  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const inputValue = e.target.value;
    
    // Allow only numbers, dots, and commas for formatting
    const cleaned = inputValue.replace(/[^\d]/g, '');
    
    // Update display with formatting
    const formatted = cleaned === '' ? '' : new Intl.NumberFormat('id-ID').format(parseInt(cleaned, 10));
    setDisplayValue(formatted);
    
    // Convert back to number and call onChange
    const numericValue = cleaned === '' ? 0 : parseInt(cleaned, 10);
    
    // Apply min/max constraints
    let constrainedValue = numericValue;
    if (min !== undefined && numericValue < min) {
      constrainedValue = min;
    }
    if (max !== undefined && numericValue > max) {
      constrainedValue = max;
    }
    
    onChange(constrainedValue);
  };

  // Handle focus
  const handleFocus = () => {
    setIsFocused(true);
    setDisplayValue(value === 0 ? '' : keepFormattedOnFocus ? formatToIDR(value) : value.toString());
  };

  // Handle blur
  const handleBlur = () => {
    setIsFocused(false);
    // Format back to currency display
    setDisplayValue(formatToIDR(value));
  };

  // Handle key press
  const handleKeyPress = (e: React.KeyboardEvent<HTMLInputElement>) => {
    // Allow only numbers, backspace, delete, arrow keys
    const allowedKeys = ['Backspace', 'Delete', 'ArrowLeft', 'ArrowRight', 'Tab'];
    const isNumber = /^\d$/.test(e.key);
    
    if (!isNumber && !allowedKeys.includes(e.key)) {
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
