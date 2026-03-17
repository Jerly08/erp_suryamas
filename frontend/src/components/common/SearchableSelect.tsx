import React, { useState, useRef, useEffect } from 'react';
import {
  Box,
  Input,
  List,
  ListItem,
  Text,
  InputGroup,
  InputRightElement,
  IconButton,
  useDisclosure,
  Spinner,
  Badge,
  HStack,
  Portal,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiChevronDown, FiChevronUp, FiX } from 'react-icons/fi';

interface SearchableSelectOption {
  id: number | string;
  code?: string;
  name: string;
  active?: boolean;
}

interface SearchableSelectProps {
  options: SearchableSelectOption[];
  value?: string | number;
  onChange: (value: string | number, option?: SearchableSelectOption) => void;
  placeholder?: string;
  isLoading?: boolean;
  isDisabled?: boolean;
  displayFormat?: (option: SearchableSelectOption) => string;
  filterFunction?: (option: SearchableSelectOption, searchTerm: string) => boolean;
  allowClear?: boolean;
  size?: 'sm' | 'md' | 'lg';
}

const SearchableSelect: React.FC<SearchableSelectProps> = ({
  options,
  value,
  onChange,
  placeholder = "Select an option...",
  isLoading = false,
  isDisabled = false,
  displayFormat = (option) => option.code ? `${option.code} - ${option.name}` : option.name,
  filterFunction = (option, searchTerm) => {
    const term = searchTerm.toLowerCase();
    return (
      option.name.toLowerCase().includes(term) ||
      (option.code && option.code.toLowerCase().includes(term))
    );
  },
  allowClear = true,
  size = 'md',
}) => {
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedOption, setSelectedOption] = useState<SearchableSelectOption | null>(null);
  const [dropdownPosition, setDropdownPosition] = useState({ top: 0, left: 0, width: 0 });
  const { isOpen, onOpen, onClose } = useDisclosure();
  const inputBg = useColorModeValue('white', 'var(--bg-secondary)');
  const inputTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const mutedTextColor = useColorModeValue('gray.500', 'var(--text-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const hoverBorderColor = useColorModeValue('gray.300', 'gray.500');
  const selectedBg = useColorModeValue('green.50', 'rgba(72, 187, 120, 0.16)');
  const selectedBorderColor = useColorModeValue('green.300', 'green.400');
  const dropdownBg = useColorModeValue('white', 'var(--bg-secondary)');
  const optionHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const optionSelectedBg = useColorModeValue('blue.50', 'rgba(66, 153, 225, 0.16)');
  const optionBorderColor = useColorModeValue('gray.100', 'var(--border-color)');
  const scrollbarTrack = useColorModeValue('#f1f1f1', 'rgba(255,255,255,0.08)');
  const scrollbarThumb = useColorModeValue('#888', '#718096');
  const scrollbarThumbHover = useColorModeValue('#555', '#A0AEC0');
  const containerRef = useRef<HTMLDivElement>(null);
  const inputRef = useRef<HTMLInputElement>(null);
  const dropdownRef = useRef<HTMLDivElement>(null);

  // Find the selected option based on value
  useEffect(() => {
    if (value) {
      const found = options.find(option => option.id.toString() === value.toString());
      if (found) {
        setSelectedOption(found);
        setSearchTerm(''); // Clear search when an option is selected
      }
    } else {
      setSelectedOption(null);
      setSearchTerm('');
    }
  }, [value, options]);

  // Filter options based on search term
  const filteredOptions = searchTerm
    ? options.filter(option => filterFunction(option, searchTerm))
    : options;

  // Handle option selection
  const handleOptionSelect = (option: SearchableSelectOption) => {
    setSelectedOption(option);
    setSearchTerm('');
    onChange(option.id, option);
    onClose();
  };

  // Handle input change (search)
  const handleInputChange = (event: React.ChangeEvent<HTMLInputElement>) => {
    const newSearchTerm = event.target.value;
    setSearchTerm(newSearchTerm);
    
    if (!isOpen && newSearchTerm) {
      onOpen();
    }
  };

  // Handle clear selection
  const handleClear = () => {
    setSelectedOption(null);
    setSearchTerm('');
    onChange('');
    onClose();
    inputRef.current?.focus();
  };

  // Handle input focus
  const handleInputFocus = () => {
    if (!isDisabled) {
      onOpen();
    }
  };

  // Close dropdown when clicking outside (both input and dropdown area)
  useEffect(() => {
    const handleClickOutside = (event: MouseEvent) => {
      const targetNode = event.target as Node;
      const containerEl = containerRef.current;
      const dropdownEl = dropdownRef.current;

      // If click happens inside the input container OR inside the dropdown portal,
      // we treat it as an internal click and do not close the dropdown.
      if ((containerEl && containerEl.contains(targetNode)) || (dropdownEl && dropdownEl.contains(targetNode))) {
        return;
      }

      onClose();
      setSearchTerm(''); // Clear search when closing
    };

    document.addEventListener('mousedown', handleClickOutside);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
    };
  }, [onClose]);

  // Smooth mouse-wheel scrolling when dropdown is open
  useEffect(() => {
    const el = dropdownRef.current;
    if (!isOpen || !el) return;

    const onWheel = (e: WheelEvent) => {
      // ignore zoom or horizontal scroll gestures
      if (e.ctrlKey || Math.abs(e.deltaX) > Math.abs(e.deltaY)) return;
      if (el.scrollHeight <= el.clientHeight) return;
      e.preventDefault();
      el.scrollBy({ top: e.deltaY, left: 0, behavior: 'smooth' });
    };

    el.addEventListener('wheel', onWheel, { passive: false });
    return () => {
      el.removeEventListener('wheel', onWheel as EventListener);
    };
  }, [isOpen]);

  // Ensure active/selected option is visible when opening
  useEffect(() => {
    if (!isOpen || !dropdownRef.current || !selectedOption) return;
    const el = dropdownRef.current.querySelector(
      `[data-option-id="${selectedOption.id}"]`
    ) as HTMLElement | null;
    el?.scrollIntoView({ block: 'nearest', behavior: 'smooth' });
  }, [isOpen, selectedOption]);

  // Update dropdown position when opening or window resizes
  useEffect(() => {
    const updatePosition = () => {
      if (containerRef.current && isOpen) {
        const rect = containerRef.current.getBoundingClientRect();
        setDropdownPosition({
          top: rect.bottom + window.scrollY,
          left: rect.left + window.scrollX,
          width: rect.width,
        });
      }
    };

    if (isOpen) {
      updatePosition();
      window.addEventListener('resize', updatePosition);
      window.addEventListener('scroll', updatePosition, true);
      return () => {
        window.removeEventListener('resize', updatePosition);
        window.removeEventListener('scroll', updatePosition, true);
      };
    }
  }, [isOpen]);

  // Display value in input
  const getInputValue = () => {
    if (searchTerm) {
      return searchTerm; // Show search term when searching
    }
    if (selectedOption) {
      return displayFormat(selectedOption); // Show selected option when not searching
    }
    return ''; // Empty when nothing selected
  };

  return (
    <Box ref={containerRef} position="relative">
      <InputGroup size={size}>
        <Input
          ref={inputRef}
          value={getInputValue()}
          onChange={handleInputChange}
          onFocus={handleInputFocus}
          placeholder={selectedOption ? '' : placeholder}
          isDisabled={isDisabled}
          bg={selectedOption ? selectedBg : inputBg}
          color={inputTextColor}
          borderColor={selectedOption ? selectedBorderColor : borderColor}
          _hover={{ borderColor: selectedOption ? selectedBorderColor : hoverBorderColor }}
          _focusVisible={{ borderColor: selectedOption ? selectedBorderColor : 'blue.400', boxShadow: selectedOption ? '0 0 0 1px var(--chakra-colors-green-400)' : '0 0 0 1px var(--chakra-colors-blue-400)' }}
          _placeholder={{ color: mutedTextColor }}
          size={size}
        />
        <InputRightElement width="4.5rem">
          <HStack spacing={1}>
            {allowClear && selectedOption && (
              <IconButton
                aria-label="Clear selection"
                icon={<FiX />}
                size="xs"
                variant="ghost"
                onClick={handleClear}
                isDisabled={isDisabled}
              />
            )}
            <IconButton
              aria-label="Toggle dropdown"
              icon={isOpen ? <FiChevronUp /> : <FiChevronDown />}
              size="xs"
              variant="ghost"
              onClick={isOpen ? onClose : onOpen}
              isDisabled={isDisabled}
            />
          </HStack>
        </InputRightElement>
      </InputGroup>

      {isOpen && !isDisabled && (
        <Portal>
          <Box
            ref={dropdownRef}
            position="fixed"
            top={`${dropdownPosition.top}px`}
            left={`${dropdownPosition.left}px`}
            width={`${dropdownPosition.width}px`}
            zIndex={2000}
            bg={dropdownBg}
            border="1px"
            borderColor={borderColor}
            borderRadius="md"
            boxShadow="xl"
            maxHeight="300px"
            overflowY="auto"
            overflowX="hidden"
            mt={1}
            // Smooth programmatic scroll and better overscroll behaviour
            sx={{ 
              scrollBehavior: 'smooth', 
              overscrollBehavior: 'contain',
              '&::-webkit-scrollbar': {
                width: '8px',
              },
              '&::-webkit-scrollbar-track': {
                background: scrollbarTrack,
                borderRadius: '4px',
              },
              '&::-webkit-scrollbar-thumb': {
                background: scrollbarThumb,
                borderRadius: '4px',
              },
              '&::-webkit-scrollbar-thumb:hover': {
                background: scrollbarThumbHover,
              },
            }}
          >
          {isLoading ? (
            <Box p={4} textAlign="center">
              <Spinner size="sm" />
              <Text ml={2} fontSize="sm" color={inputTextColor}>Loading...</Text>
            </Box>
          ) : filteredOptions.length > 0 ? (
            <List>
              {filteredOptions.map((option) => (
                <ListItem
                  key={option.id}
                  data-option-id={option.id}
                  p={3}
                  cursor="pointer"
                  _hover={{ bg: optionHoverBg }}
                  bg={selectedOption?.id === option.id ? optionSelectedBg : dropdownBg}
                  borderBottom="1px"
                  borderColor={optionBorderColor}
                  onClick={() => handleOptionSelect(option)}
                >
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={inputTextColor}>
                      {displayFormat(option)}
                    </Text>
                    {option.active === false && (
                      <Badge colorScheme="red" size="sm">
                        Inactive
                      </Badge>
                    )}
                  </HStack>
                </ListItem>
              ))}
            </List>
          ) : (
            <Box p={4} textAlign="center">
              <Text fontSize="sm" color={mutedTextColor}>
                {searchTerm ? 'No results found' : 'No options available'}
              </Text>
            </Box>
          )}
          </Box>
        </Portal>
      )}
    </Box>
  );
};

export default SearchableSelect;
