'use client';

import React from 'react';
import {
  Table as ChakraTable,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Card,
  CardHeader,
  CardBody,
  Heading,
  Spinner,
  Flex,
  Text,
  Badge,
  Button,
  Box,
  useColorModeValue,
} from '@chakra-ui/react';

interface Column<T> {
  header: string;
  accessor: string | ((item: T) => React.ReactNode);
  cell?: (item: T) => React.ReactNode;
}

interface TableProps<T> {
  columns: Column<T>[];
  data: T[];
  keyField: keyof T;
  title?: string;
  actions?: (item: T) => React.ReactNode;
  isLoading?: boolean;
  emptyMessage?: string;
}

function Table<T>({ columns, data, keyField, title, actions, isLoading, emptyMessage }: TableProps<T>) {
  const bgCard = useColorModeValue('white', 'gray.800');
  const bgHeader = useColorModeValue('#F7FAFC', 'gray.700');
  const borderColor = useColorModeValue('gray.200', 'gray.600');
  const hoverBg = useColorModeValue('gray.50', 'gray.700');
  const textColor = useColorModeValue('gray.700', 'gray.200');
  const headerTextColor = useColorModeValue('gray.600', 'gray.300');
  const mutedTextColor = useColorModeValue('gray.500', 'gray.400');
  
  const renderCell = (item: T, column: Column<T>) => {
    if (column.cell) {
      return column.cell(item);
    }
    
    if (typeof column.accessor === 'function') {
      return column.accessor(item);
    }
    
    return item[column.accessor as keyof T] as React.ReactNode;
  };

  const getStatusColor = (status: string) => {
    switch (status?.toLowerCase()) {
      case 'active':
        return 'green';
      case 'inactive':
        return 'red';
      case 'pending':
        return 'yellow';
      case 'approved':
        return 'blue';
      default:
        return 'gray';
    }
  };

  const renderCellContent = (content: React.ReactNode) => {
    if (typeof content === 'string') {
      // Check if it's a status-like field
      if (['active', 'inactive', 'pending', 'approved'].includes(content.toLowerCase())) {
        return (
          <Badge colorScheme={getStatusColor(content)} variant="subtle">
            {content}
          </Badge>
        );
      }
    }
    return content;
  };

  if (isLoading) {
    return (
      <Card bg={bgCard} borderColor={borderColor} variant="outline">
        {title && (
          <CardHeader borderBottomWidth="1px" borderColor={borderColor}>
            <Heading size="md" color={textColor}>{title}</Heading>
          </CardHeader>
        )}
        <CardBody>
          <Flex justify="center" align="center" py={8}>
            <Spinner size="lg" color="brand.500" />
            <Text ml={4} color={mutedTextColor}>Loading...</Text>
          </Flex>
        </CardBody>
      </Card>
    );
  }

  return (
    <Card 
      bg={bgCard} 
      borderRadius="lg" 
      boxShadow="sm" 
      border="1px" 
      borderColor={borderColor}
      overflow="hidden"
    >
      {title && (
        <CardHeader pb={0}>
          <Heading size="md" color={textColor}>{title}</Heading>
        </CardHeader>
      )}
      <CardBody p={0}>
        {data.length === 0 ? (
          <Flex justify="center" align="center" py={8}>
            <Text color={mutedTextColor}>{emptyMessage || 'No data available'}</Text>
          </Flex>
        ) : (
          <Box overflowX="auto">
            <ChakraTable variant="simple" size="md">
              <Thead bg={bgHeader}>
                <Tr>
                  {columns.map((column, index) => (
                    <Th 
                      key={index} 
                      fontWeight="500"
                      textTransform="none"
                      fontSize="sm"
                      color={headerTextColor}
                      px={6}
                      py={4}
                      borderBottom="1px"
                      borderColor={borderColor}
                    >
                      {column.header}
                    </Th>
                  ))}
                  {actions && (
                    <Th 
                      fontWeight="500"
                      textTransform="none"
                      fontSize="sm"
                      color={headerTextColor}
                      px={6}
                      py={4}
                      borderBottom="1px"
                      borderColor={borderColor}
                      textAlign="center"
                    >
                      Actions
                    </Th>
                  )}
                </Tr>
              </Thead>
              <Tbody>
                {data.map((item, rowIndex) => (
                  <Tr 
                    key={String(item[keyField])} 
                    _hover={{ bg: hoverBg, cursor: 'pointer' }}
                    transition="all 0.2s"
                  >
                    {columns.map((column, index) => (
                      <Td 
                        key={index}
                        px={6}
                        py={4}
                        fontSize="sm"
                        color={textColor}
                        borderBottom={rowIndex === data.length - 1 ? 'none' : '1px'}
                        borderColor={borderColor}
                      >
                        {renderCellContent(renderCell(item, column))}
                      </Td>
                    ))}
                    {actions && (
                      <Td
                        px={6}
                        py={4}
                        borderBottom={rowIndex === data.length - 1 ? 'none' : '1px'}
                        borderColor={borderColor}
                      >
                        <Flex gap={2} justify="center" wrap="wrap">
                          {actions(item)}
                        </Flex>
                      </Td>
                    )}
                  </Tr>
                ))}
              </Tbody>
            </ChakraTable>
          </Box>
        )}
      </CardBody>
    </Card>
  );
}

export default Table;
