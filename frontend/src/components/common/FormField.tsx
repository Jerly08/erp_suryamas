'use client';

import React from 'react';
import {
  FormControl,
  FormErrorMessage,
  FormLabel,
  Input,
  Select,
  Textarea,
  useColorModeValue,
} from '@chakra-ui/react';

interface FormFieldProps {
  id: string;
  label: string;
  type?: 'text' | 'email' | 'password' | 'number' | 'date' | 'textarea' | 'select';
  value: string | number;
  onChange: (e: React.ChangeEvent<HTMLInputElement | HTMLTextAreaElement | HTMLSelectElement>) => void;
  placeholder?: string;
  required?: boolean;
  error?: string;
  options?: { value: string | number; label: string }[];
  className?: string;
  disabled?: boolean;
  name?: string;
}

const FormField: React.FC<FormFieldProps> = ({
  id,
  label,
  type = 'text',
  value,
  onChange,
  placeholder,
  required = false,
  error,
  options,
  className = '',
  disabled = false,
  name,
}) => {
  const labelColor = useColorModeValue('gray.700', 'var(--text-primary)');
  const inputBg = useColorModeValue('white', 'var(--bg-secondary)');
  const inputTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const placeholderColor = useColorModeValue('gray.400', 'var(--text-secondary)');
  const borderColor = useColorModeValue(error ? 'red.500' : 'gray.300', error ? 'red.300' : 'var(--border-color)');
  const hoverBorderColor = useColorModeValue(error ? 'red.500' : 'gray.400', error ? 'red.300' : 'gray.500');
  const disabledBg = useColorModeValue('gray.100', 'whiteAlpha.100');

  const commonFieldProps = {
    name: name || id,
    value,
    onChange,
    placeholder,
    isRequired: required,
    isDisabled: disabled,
    bg: disabled ? disabledBg : inputBg,
    color: inputTextColor,
    borderColor,
    _hover: { borderColor: hoverBorderColor },
    _focusVisible: {
      borderColor: error ? 'red.500' : 'blue.400',
      boxShadow: error ? '0 0 0 1px var(--chakra-colors-red-500)' : '0 0 0 1px var(--chakra-colors-blue-400)',
    },
    _placeholder: { color: placeholderColor },
  };

  const renderField = () => {
    switch (type) {
      case 'textarea':
        return (
          <Textarea
            id={id}
            minH="96px"
            {...commonFieldProps}
          />
        );
      case 'select':
        return (
          <Select
            id={id}
            {...commonFieldProps}
          >
            <option value="">Select an option</option>
            {options?.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </Select>
        );
      default:
        return (
          <Input
            id={id}
            type={type}
            {...commonFieldProps}
          />
        );
    }
  };

  return (
    <FormControl className={className} mb={4} isInvalid={!!error}>
      <FormLabel htmlFor={id} fontSize="sm" fontWeight="medium" mb={1} color={labelColor}>
        {label}
      </FormLabel>
      {renderField()}
      {error && <FormErrorMessage fontSize="xs">{error}</FormErrorMessage>}
    </FormControl>
  );
};

export default FormField; 
