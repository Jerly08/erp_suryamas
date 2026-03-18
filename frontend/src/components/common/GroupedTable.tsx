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
  Box,
  IconButton,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiChevronDown, FiChevronRight } from 'react-icons/fi';
import { useTranslation } from '@/hooks/useTranslation';

interface Column<T> {
  header: string;
  accessor: string | ((item: T) => React.ReactNode);
  cell?: (item: T) => React.ReactNode;
  headerStyle?: Record<string, any>;
  cellStyle?: Record<string, any>;
}

interface Group<T> {
  key: string;
  items: T[];
  isOpen: boolean;
}

interface GroupedTableProps<T> {
  columns: Column<T>[] | ((groupKey: string) => Column<T>[]);
  data: T[];
  keyField: keyof T;
  groupBy: keyof T;
  title?: string;
  actions?: (item: T) => React.ReactNode;
  isLoading?: boolean;
  emptyMessage?: string;
  groupLabels?: Record<string, string>;
}

function GroupedTable<T>({
  columns,
  data,
  keyField,
  groupBy,
  title,
  actions,
  isLoading,
  emptyMessage,
  groupLabels = {}
}: GroupedTableProps<T>) {
  const { t } = useTranslation();
  const [groups, setGroups] = React.useState<Record<string, Group<T>>>({});
  const cardBg = useColorModeValue('white', 'gray.800');
  const cardBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const mutedTextColor = useColorModeValue('gray.500', 'whiteAlpha.700');
  const groupHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const groupHeaderHoverBg = useColorModeValue('gray.100', 'whiteAlpha.200');
  const groupHeaderTextColor = useColorModeValue('gray.700', 'whiteAlpha.900');
  const tableHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const tableHeaderTextColor = useColorModeValue('gray.700', 'whiteAlpha.800');
  const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const tableBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');

  React.useEffect(() => {
    // Group data by the specified field
    const groupedData = data.reduce((acc, item) => {
      const groupKey = String(item[groupBy]);
      if (!acc[groupKey]) {
        acc[groupKey] = {
          key: groupKey,
          items: [],
          isOpen: true // Groups are open by default
        };
      }
      acc[groupKey].items.push(item);
      return acc;
    }, {} as Record<string, Group<T>>);

    setGroups(groupedData);
  }, [data, groupBy]);

  const toggleGroup = (groupKey: string) => {
    setGroups(prev => ({
      ...prev,
      [groupKey]: {
        ...prev[groupKey],
        isOpen: !prev[groupKey]?.isOpen
      }
    }));
  };

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

  const getGroupColor = (groupKey: string) => {
    switch (groupKey) {
      case 'VENDOR':
        return 'purple';
      case 'CUSTOMER':
        return 'blue';
      case 'EMPLOYEE':
        return 'green';
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
      <Card bg={cardBg} borderColor={cardBorderColor} variant="outline">
        {title && (
          <CardHeader borderBottomWidth="1px" borderColor={cardBorderColor}>
            <Heading size="md" color={headingColor}>
              {title}
            </Heading>
          </CardHeader>
        )}
        <CardBody>
          <Flex justify="center" align="center" py={8}>
            <Spinner size="lg" color="brand.500" />
            <Text ml={4} color={mutedTextColor}>
              {t('common.loading')}
            </Text>
          </Flex>
        </CardBody>
      </Card>
    );
  }

  const sortedGroups = Object.values(groups).sort((a, b) => a.key.localeCompare(b.key));

  return (
    <Card bg={cardBg} borderColor={cardBorderColor} variant="outline">
      {title && (
        <CardHeader borderBottomWidth="1px" borderColor={cardBorderColor}>
          <Heading size="md" color={headingColor}>
            {title}
          </Heading>
        </CardHeader>
      )}
      <CardBody p={0}>
        {data.length === 0 ? (
          <Flex justify="center" align="center" py={8}>
            <Text color={mutedTextColor}>{emptyMessage || t('common.noData')}</Text>
          </Flex>
        ) : (
          <Box>
            {sortedGroups.map((group) => {
              const groupColumns = typeof columns === 'function' ? columns(group.key) : columns;

              return (
                <Box key={group.key} mb={4}>
                  {/* Group Header */}
                  <Flex
                    align="center"
                    px={4}
                    py={3}
                    bg={groupHeaderBg}
                    cursor="pointer"
                    onClick={() => toggleGroup(group.key)}
                    _hover={{ bg: groupHeaderHoverBg }}
                    borderBottom="1px solid"
                    borderColor={tableBorderColor}
                  >
                    <IconButton
                      aria-label={group.isOpen ? 'Collapse group' : 'Expand group'}
                      icon={group.isOpen ? <FiChevronDown /> : <FiChevronRight />}
                      size="sm"
                      variant="ghost"
                      color={groupHeaderTextColor}
                      mr={2}
                    />
                    <Badge
                      colorScheme={getGroupColor(group.key)}
                      variant="solid"
                      mr={3}
                      px={2}
                      py={1}
                    >
                      {groupLabels[group.key] || group.key}
                    </Badge>
                    <Text fontWeight="semibold" fontSize="md" color={groupHeaderTextColor}>
                      {group.items.length} {group.items.length === 1 ? t('common.item') : t('common.items')}
                    </Text>
                  </Flex>

                  {/* Group Content */}
                  {group.isOpen && (
                    <Box overflowX="auto">
                      <ChakraTable variant="simple" size="sm">
                        <Thead bg={tableHeaderBg}>
                          <Tr>
                            {groupColumns.map((column, index) => (
                              <Th
                                key={index}
                                fontWeight="bold"
                                whiteSpace="nowrap"
                                px={4}
                                py={3}
                                fontSize="sm"
                                color={tableHeaderTextColor}
                                borderColor={tableBorderColor}
                                {...(column.headerStyle || {})}
                              >
                                {column.header}
                              </Th>
                            ))}
                            {actions && (
                              <Th
                                whiteSpace="nowrap"
                                px={4}
                                py={3}
                                fontSize="sm"
                                color={tableHeaderTextColor}
                                borderColor={tableBorderColor}
                              >
                                {t('common.actions')}
                              </Th>
                            )}
                          </Tr>
                        </Thead>
                        <Tbody>
                          {group.items.map((item) => (
                            <Tr key={String(item[keyField])} _hover={{ bg: rowHoverBg }}>
                              {groupColumns.map((column, index) => (
                                <Td
                                  key={index}
                                  px={4}
                                  py={3}
                                  fontSize="sm"
                                  verticalAlign="middle"
                                  borderColor={tableBorderColor}
                                  {...(column.cellStyle || {})}
                                >
                                  {renderCellContent(renderCell(item, column))}
                                </Td>
                              ))}
                              {actions && (
                                <Td
                                  px={4}
                                  py={3}
                                  verticalAlign="middle"
                                  borderColor={tableBorderColor}
                                >
                                  <Flex gap={2} justify="flex-end" wrap="wrap">
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
                </Box>
              );
            })}
          </Box>
        )}
      </CardBody>
    </Card>
  );
}

export default GroupedTable;
