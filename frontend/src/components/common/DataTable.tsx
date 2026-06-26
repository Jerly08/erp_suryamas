'use client';

import React, { useState, useEffect } from 'react';
import {
  Box,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Input,
  InputGroup,
  InputLeftElement,
  Button,
  Flex,
  Text,
  HStack,
  Heading,
  useColorModeValue,
} from '@chakra-ui/react';
import { SearchIcon } from '@chakra-ui/icons';
import { useTranslation } from '@/hooks/useTranslation';

interface Column<T> {
  header: string;
  accessor: keyof T | ((row: T) => React.ReactNode);
  className?: string;
}

interface DataTableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyField: keyof T;
  title?: string;
  searchable?: boolean;
  pagination?: boolean;
  pageSize?: number;
  actions?: (row: T) => React.ReactNode;
  onRowClick?: (row: T) => void;
}

function DataTable<T>({
  columns,
  data,
  keyField,
  title,
  searchable = true,
  pagination = true,
  pageSize = 10,
  actions,
  onRowClick,
}: DataTableProps<T>) {
  const { t } = useTranslation();
  const [searchTerm, setSearchTerm] = useState('');
  const [currentPage, setCurrentPage] = useState(1);
  const [filteredData, setFilteredData] = useState<T[]>(data);
  const [paginatedData, setPaginatedData] = useState<T[]>([]);

  // Color mode values
  const bg = useColorModeValue('white', 'gray.800');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const headerBg = useColorModeValue('gray.50', 'gray.700');
  const hoverBg = useColorModeValue('gray.50', 'gray.700');
  const textColor = useColorModeValue('gray.900', 'white');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
  const headingColor = useColorModeValue('gray.800', 'white');
  const inputBg = useColorModeValue('white', 'gray.700');
  const buttonBg = useColorModeValue('gray.200', 'gray.600');
  const buttonHoverBg = useColorModeValue('gray.300', 'gray.500');
  const buttonDisabledBg = useColorModeValue('gray.100', 'gray.700');
  const buttonDisabledColor = useColorModeValue('gray.400', 'gray.500');
  const activeBg = useColorModeValue('blue.600', 'blue.500');

  // Filter data based on search term
  useEffect(() => {
    if (!searchable || searchTerm === '') {
      setFilteredData(data);
    } else {
      const filtered = data.filter((row) => {
        return columns.some((column) => {
          if (typeof column.accessor === 'function') {
            return false; // Skip function accessors for search
          }
          
          const value = row[column.accessor as keyof T];
          if (value === null || value === undefined) return false;
          
          return String(value).toLowerCase().includes(searchTerm.toLowerCase());
        });
      });
      setFilteredData(filtered);
    }
    setCurrentPage(1);
  }, [searchTerm, data, columns, searchable]);

  // Paginate data
  useEffect(() => {
    if (!pagination) {
      setPaginatedData(filteredData);
      return;
    }
    
    const startIndex = (currentPage - 1) * pageSize;
    const endIndex = startIndex + pageSize;
    setPaginatedData(filteredData.slice(startIndex, endIndex));
  }, [filteredData, currentPage, pageSize, pagination]);

  // Calculate total pages
  const totalPages = Math.ceil(filteredData.length / pageSize);

  // Handle page change
  const handlePageChange = (page: number) => {
    setCurrentPage(page);
  };

  // Render cell content
  const renderCell = (row: T, column: Column<T>) => {
    if (typeof column.accessor === 'function') {
      return column.accessor(row);
    }
    
    const value = row[column.accessor as keyof T];
    return value !== null && value !== undefined ? String(value) : '';
  };

  return (
    <Box bg={bg} shadow="md" borderRadius="lg" overflow="hidden" borderWidth="1px" borderColor={borderColor} maxW="full">
      {/* Header with title and search */}
      <Flex
        p={{ base: 3, md: 4 }}
        borderBottomWidth="1px"
        borderColor={borderColor}
        justify="space-between"
        align={{ base: 'stretch', md: 'center' }}
        direction={{ base: 'column', md: 'row' }}
        gap={3}
      >
        {title && <Heading size={{ base: 'md', md: 'lg' }} color={headingColor}>{title}</Heading>}
        
        {searchable && (
          <InputGroup maxW={{ base: 'full', md: '300px' }}>
            <InputLeftElement pointerEvents="none">
              <SearchIcon color={mutedTextColor} />
            </InputLeftElement>
            <Input
              type="text"
              placeholder={t('common.placeholders.search')}
              value={searchTerm}
              onChange={(e) => setSearchTerm(e.target.value)}
              bg={inputBg}
              borderColor={borderColor}
              color={textColor}
              _focus={{
                borderColor: 'blue.500',
                boxShadow: '0 0 0 1px #3182ce'
              }}
            />
          </InputGroup>
        )}
      </Flex>
      
      {/* Table */}
      <Box overflowX="auto" maxW="full" className="mobile-scroll-x">
        <Table variant="simple" size={{ base: 'sm', md: 'md' }} minW={{ base: '720px', md: 'full' }}>
          <Thead>
            <Tr bg={headerBg}>
              {columns.map((column, index) => (
                <Th
                  key={index}
                  color={mutedTextColor}
                  fontWeight="500"
                  fontSize="xs"
                  textTransform="uppercase"
                  letterSpacing={0}
                  borderColor={borderColor}
                  px={{ base: 3, md: 4 }}
                  py={{ base: 3, md: 4 }}
                >
                  {column.header}
                </Th>
              ))}
              {actions && (
                <Th
                  textAlign="right"
                  color={mutedTextColor}
                  borderColor={borderColor}
                  px={{ base: 3, md: 4 }}
                  py={{ base: 3, md: 4 }}
                >
                  {t('common.actions')}
                </Th>
              )}
            </Tr>
          </Thead>
          <Tbody>
            {paginatedData.length > 0 ? (
              paginatedData.map((row) => (
                <Tr
                  key={String(row[keyField])}
                  _hover={{ bg: hoverBg }}
                  cursor={onRowClick ? 'pointer' : 'default'}
                  onClick={() => onRowClick && onRowClick(row)}
                >
                  {columns.map((column, index) => (
                    <Td
                      key={index}
                      color={textColor}
                      fontSize="sm"
                      borderColor={borderColor}
                      px={{ base: 3, md: 4 }}
                      py={{ base: 3, md: 4 }}
                    >
                      {renderCell(row, column)}
                    </Td>
                  ))}
                  {actions && (
                    <Td
                      textAlign="right"
                      fontSize="sm"
                      borderColor={borderColor}
                      px={{ base: 3, md: 4 }}
                      py={{ base: 3, md: 4 }}
                    >
                      {actions(row)}
                    </Td>
                  )}
                </Tr>
              ))
            ) : (
              <Tr>
                <Td
                  colSpan={columns.length + (actions ? 1 : 0)}
                  textAlign="center"
                  fontSize="sm"
                  color={mutedTextColor}
                  borderColor={borderColor}
                  px={{ base: 3, md: 4 }}
                  py={{ base: 4, md: 6 }}
                >
                  {t('common.table.noData')}
                </Td>
              </Tr>
            )}
          </Tbody>
        </Table>
      </Box>
      
      {/* Pagination */}
      {pagination && totalPages > 1 && (
        <Flex
          px={{ base: 3, md: 4 }}
          py={3}
          borderTopWidth="1px"
          borderColor={borderColor}
          justify="space-between"
          align={{ base: 'stretch', md: 'center' }}
          direction={{ base: 'column', md: 'row' }}
          gap={3}
        >
          <Text fontSize="sm" color={mutedTextColor}>
            {t('common.pagination.showing')} {((currentPage - 1) * pageSize) + 1} {t('common.pagination.to')} {Math.min(currentPage * pageSize, filteredData.length)} {t('common.pagination.of')} {filteredData.length} {t('common.pagination.entries')}
          </Text>
          
          <HStack spacing={1} overflowX="auto" maxW="full" pb={{ base: 1, md: 0 }}>
            <Button
              onClick={() => handlePageChange(currentPage - 1)}
              isDisabled={currentPage === 1}
              size="sm"
              bg={currentPage === 1 ? buttonDisabledBg : buttonBg}
              color={currentPage === 1 ? buttonDisabledColor : textColor}
              _hover={currentPage === 1 ? {} : { bg: buttonHoverBg }}
              cursor={currentPage === 1 ? 'not-allowed' : 'pointer'}
            >
              {t('common.previous')}
            </Button>
            
            {Array.from({ length: totalPages }, (_, i) => i + 1).map((page) => (
              <Button
                key={page}
                onClick={() => handlePageChange(page)}
                size="sm"
                bg={currentPage === page ? activeBg : buttonBg}
                color={currentPage === page ? 'white' : textColor}
                _hover={currentPage === page ? {} : { bg: buttonHoverBg }}
              >
                {page}
              </Button>
            ))}
            
            <Button
              onClick={() => handlePageChange(currentPage + 1)}
              isDisabled={currentPage === totalPages}
              size="sm"
              bg={currentPage === totalPages ? buttonDisabledBg : buttonBg}
              color={currentPage === totalPages ? buttonDisabledColor : textColor}
              _hover={currentPage === totalPages ? {} : { bg: buttonHoverBg }}
              cursor={currentPage === totalPages ? 'not-allowed' : 'pointer'}
            >
              {t('common.next')}
            </Button>
          </HStack>
        </Flex>
      )}
    </Box>
  );
}

export { DataTable };
export default DataTable;
