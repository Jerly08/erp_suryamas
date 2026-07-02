'use client';

import React, { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { contactService } from '@/services/contactService';
import { Contact } from '@/types/contact';
import {
  Box,
  Flex,
  Heading,
  Button,
  Text,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  Badge,
  HStack,
  VStack,
  SimpleGrid,
  Card,
  CardHeader,
  CardBody,
  Stat,
  StatLabel,
  StatNumber,
  Icon,
  useColorModeValue,
  Input,
  Select,
  InputGroup,
  InputLeftElement,
  Spinner,
  Alert,
  AlertIcon,
  useToast,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  IconButton,
  Tooltip,
} from '@chakra-ui/react';
import { 
  FiPlus, 
  FiEye, 
  FiEdit, 
  FiTrash2,
  FiDownload, 
  FiUpload, 
  FiSearch, 
  FiBarChart,
  FiFilter,
  FiRefreshCw,
  FiMoreVertical,
  FiPhone,
  FiMail,
  FiMapPin,
  FiUser,
  FiHome,
  FiUsers
} from 'react-icons/fi';

export default function EnhancedTableContactsPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();
  
  // State management
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [filterType, setFilterType] = useState('ALL');
  const [currentPage, setCurrentPage] = useState(1);
  const [itemsPerPage] = useState(10);
  
  // Theme-aware colors
  const headingColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const subheadingColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const tableBg = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const primaryTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const hoverBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');
  const theadBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');

  // Load contacts from backend
  const loadContacts = async () => {
    try {
      setLoading(true);
      const authToken = token;
      if (!authToken) {
        return;
      }

      let contactsData: Contact[];
      if (filterType === 'ALL') {
        contactsData = await contactService.getContacts(authToken);
      } else {
        contactsData = await contactService.getContacts(authToken, filterType);
      }
      
      // Apply search filter
      if (searchQuery) {
        contactsData = contactsData.filter(contact =>
          contact.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (contact.email && contact.email.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.phone && contact.phone.includes(searchQuery)) ||
          (contact.mobile && contact.mobile.includes(searchQuery)) ||
          (contact.code && contact.code.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.pic_name && contact.pic_name.toLowerCase().includes(searchQuery.toLowerCase()))
        );
      }
      
      setContacts(contactsData);
    } catch (error) {
      console.error('Error loading contacts:', error);
      toast({
        title: 'Error Loading Contacts',
        description: error instanceof Error ? error.message : 'Unknown error occurred',
        status: 'error',
        duration: 5000,
        isClosable: true,
      });
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    loadContacts();
  }, [filterType, searchQuery, token]);

  // Calculate statistics
  const stats = {
    total: contacts.length,
    active: contacts.filter(c => c.is_active).length,
    customers: contacts.filter(c => c.type === 'CUSTOMER').length,
    vendors: contacts.filter(c => c.type === 'VENDOR').length,
    employees: contacts.filter(c => c.type === 'EMPLOYEE').length,
  };

  // Pagination
  const indexOfLastItem = currentPage * itemsPerPage;
  const indexOfFirstItem = indexOfLastItem - itemsPerPage;
  const currentContacts = contacts.slice(indexOfFirstItem, indexOfLastItem);
  const totalPages = Math.ceil(contacts.length / itemsPerPage);

  // Get contact type badge color
  const getTypeColor = (type: string) => {
    switch (type) {
      case 'CUSTOMER': return 'blue';
      case 'VENDOR': return 'green';
      case 'EMPLOYEE': return 'purple';
      default: return 'gray';
    }
  };

  // Get contact type icon
  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'CUSTOMER': return FiUser;
      case 'VENDOR': return FiHome;
      case 'EMPLOYEE': return FiUsers;
      default: return FiUser;
    }
  };

  return (
    <SimpleLayout allowedRoles={['admin', 'finance', 'employee', 'director']}>
      <Box>
        {/* Header */}
        <Flex justify="space-between" align="center" mb={6} wrap="wrap" gap={4}>
          <VStack align="start" spacing={1}>
            <Heading size="xl" color={headingColor} fontWeight="600">
              Contact Master
            </Heading>
            <Text color={subheadingColor} fontSize="md">
              Manage your business contacts and relationships
            </Text>
          </VStack>
          
          <HStack spacing={3}>
            <Tooltip label="Refresh Data">
              <IconButton
                aria-label="Refresh"
                icon={<FiRefreshCw />}
                variant="ghost"
                onClick={loadContacts}
                isLoading={loading}
              />
            </Tooltip>
            
            <Button
              colorScheme="blue"
              leftIcon={<FiPlus />}
              size="md"
              px={6}
              fontWeight="medium"
              onClick={onOpen}
              _hover={{ 
                transform: 'translateY(-1px)',
                boxShadow: 'lg'
              }}
            >
              Add New Contact
            </Button>
          </HStack>
        </Flex>

        {/* Statistics Cards */}
        <SimpleGrid columns={{ base: 1, md: 2, lg: 5 }} spacing={4} mb={6}>
          <Card>
            <CardBody>
              <Stat>
                <StatLabel color={textColor} fontSize="sm" textTransform="uppercase" letterSpacing="wider">
                  Total Contacts
                </StatLabel>
                <HStack>
                  <Icon as={FiUsers} color="var(--accent-color)" />
                  <StatNumber color="var(--accent-color)" fontSize="2xl" fontWeight="bold">
                    {stats.total}
                  </StatNumber>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
          
          <Card>
            <CardBody>
              <Stat>
                <StatLabel color={textColor} fontSize="sm" textTransform="uppercase" letterSpacing="wider">
                  Active Contacts
                </StatLabel>
                <HStack>
                  <Icon as={FiUser} color="var(--success-color)" />
                  <StatNumber color="var(--success-color)" fontSize="2xl" fontWeight="bold">
                    {stats.active}
                  </StatNumber>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
          
          <Card>
            <CardBody>
              <Stat>
                <StatLabel color={textColor} fontSize="sm" textTransform="uppercase" letterSpacing="wider">
                  Customers
                </StatLabel>
                <HStack>
                  <Icon as={FiUser} color="blue.500" />
                  <StatNumber color="blue.500" fontSize="2xl" fontWeight="bold">
                    {stats.customers}
                  </StatNumber>
                </HStack>
              </Stat>
            </CardBody>
          </Card>

          <Card>
            <CardBody>
              <Stat>
                <StatLabel color={textColor} fontSize="sm" textTransform="uppercase" letterSpacing="wider">
                  Vendors
                </StatLabel>
                <HStack>
                  <Icon as={FiHome} color="green.500" />
                  <StatNumber color="green.500" fontSize="2xl" fontWeight="bold">
                    {stats.vendors}
                  </StatNumber>
                </HStack>
              </Stat>
            </CardBody>
          </Card>

          <Card>
            <CardBody>
              <Stat>
                <StatLabel color={textColor} fontSize="sm" textTransform="uppercase" letterSpacing="wider">
                  Employees
                </StatLabel>
                <HStack>
                  <Icon as={FiUsers} color="purple.500" />
                  <StatNumber color="purple.500" fontSize="2xl" fontWeight="bold">
                    {stats.employees}
                  </StatNumber>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </SimpleGrid>

        {/* Filters */}
        <Card mb={6}>
          <CardBody>
            <Flex gap={4} align="end" wrap="wrap">
              <Box flex="1" minW="300px">
                <Text fontSize="sm" fontWeight="medium" mb={2} color={textColor}>
                  Search Contacts
                </Text>
                <InputGroup>
                  <InputLeftElement pointerEvents="none">
                    <FiSearch color={textColor} />
                  </InputLeftElement>
                  <Input
                    placeholder="Search by name, email, phone, or PIC..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    bg={tableBg}
                  />
                </InputGroup>
              </Box>
              
              <Box minW="200px">
                <Text fontSize="sm" fontWeight="medium" mb={2} color={textColor}>
                  Filter by Type
                </Text>
                <Select
                  value={filterType}
                  onChange={(e) => setFilterType(e.target.value)}
                  bg={tableBg}
                >
                  <option value="ALL">All Types</option>
                  <option value="CUSTOMER">Customers</option>
                  <option value="VENDOR">Vendors</option>
                  <option value="EMPLOYEE">Employees</option>
                </Select>
              </Box>
              
              <Button
                leftIcon={<FiFilter />}
                variant="outline"
                onClick={() => {
                  setSearchQuery('');
                  setFilterType('ALL');
                }}
              >
                Clear Filters
              </Button>
            </Flex>
          </CardBody>
        </Card>

        {/* Contacts Table */}
        <Card mb={6}>
          <CardHeader>
            <Flex justify="space-between" align="center">
              <Heading size="md" color={primaryTextColor}>
                Contacts ({contacts.length})
              </Heading>
              
              <HStack spacing={2}>
                <Button leftIcon={<FiDownload />} size="sm" variant="outline">
                  Export
                </Button>
                <Button leftIcon={<FiUpload />} size="sm" variant="outline">
                  Import
                </Button>
              </HStack>
            </Flex>
          </CardHeader>
          <CardBody p={0}>
            {loading ? (
              <Flex justify="center" align="center" py={10}>
                <Spinner size="lg" color="var(--accent-color)" />
                <Text ml={4} color={textColor}>Loading contacts...</Text>
              </Flex>
            ) : contacts.length === 0 ? (
              <Alert status="info" variant="subtle">
                <AlertIcon />
                <Box>
                  <Text fontWeight="medium">No contacts found</Text>
                  <Text fontSize="sm" color={textColor}>
                    Try adjusting your search criteria or add a new contact.
                  </Text>
                </Box>
              </Alert>
            ) : (
              <Box overflowX="auto">
                <Table size="md" className="table">
                  <Thead bg={theadBg}>
                    <Tr>
                      <Th color={textColor} borderColor={borderColor}>Type</Th>
                      <Th color={textColor} borderColor={borderColor}>Name</Th>
                      <Th color={textColor} borderColor={borderColor}>Code</Th>
                      <Th color={textColor} borderColor={borderColor}>Contact Info</Th>
                      <Th color={textColor} borderColor={borderColor}>PIC</Th>
                      <Th color={textColor} borderColor={borderColor}>Status</Th>
                      <Th color={textColor} borderColor={borderColor} textAlign="center">Actions</Th>
                    </Tr>
                  </Thead>
                  <Tbody>
                    {currentContacts.map((contact) => (
                      <Tr key={contact.id} _hover={{ bg: hoverBg }} transition="all 0.2s ease">
                        <Td borderColor={borderColor}>
                          <HStack>
                            <Icon as={getTypeIcon(contact.type)} color={`${getTypeColor(contact.type)}.500`} />
                            <Badge 
                              colorScheme={getTypeColor(contact.type)}
                              variant="subtle"
                              fontSize="xs"
                            >
                              {contact.type}
                            </Badge>
                          </HStack>
                        </Td>
                        <Td borderColor={borderColor}>
                          <VStack align="start" spacing={1}>
                            <Text fontWeight="500" color={primaryTextColor}>
                              {contact.name}
                            </Text>
                            {contact.external_id && (
                              <Text fontSize="xs" color={textColor} fontFamily="monospace">
                                ID: {contact.external_id}
                              </Text>
                            )}
                          </VStack>
                        </Td>
                        <Td borderColor={borderColor}>
                          <Text fontFamily="monospace" color={textColor} fontSize="sm">
                            {contact.code}
                          </Text>
                        </Td>
                        <Td borderColor={borderColor}>
                          <VStack align="start" spacing={1}>
                            {contact.email && (
                              <HStack spacing={2}>
                                <Icon as={FiMail} color={textColor} size="12px" />
                                <Text fontSize="sm" color="var(--accent-color)" _hover={{ textDecoration: 'underline' }}>
                                  {contact.email}
                                </Text>
                              </HStack>
                            )}
                            {contact.phone && (
                              <HStack spacing={2}>
                                <Icon as={FiPhone} color={textColor} size="12px" />
                                <Text fontSize="sm" color={textColor}>
                                  {contact.phone}
                                </Text>
                              </HStack>
                            )}
                            {contact.address && (
                              <HStack spacing={2}>
                                <Icon as={FiMapPin} color={textColor} size="12px" />
                                <Text fontSize="sm" color={textColor} noOfLines={1} maxW="200px">
                                  {contact.address}
                                </Text>
                              </HStack>
                            )}
                          </VStack>
                        </Td>
                        <Td borderColor={borderColor}>
                          <Text color={textColor} fontSize="sm">
                            {contact.pic_name || '-'}
                          </Text>
                        </Td>
                        <Td borderColor={borderColor}>
                          <Badge 
                            colorScheme={contact.is_active ? 'green' : 'gray'}
                            variant="subtle"
                            className={contact.is_active ? 'badge-success' : 'badge-danger'}
                          >
                            {contact.is_active ? 'ACTIVE' : 'INACTIVE'}
                          </Badge>
                        </Td>
                        <Td borderColor={borderColor}>
                          <Menu>
                            <MenuButton
                              as={IconButton}
                              icon={<FiMoreVertical />}
                              variant="ghost"
                              size="sm"
                            />
                            <MenuList>
                              <MenuItem icon={<FiEye />}>
                                View Details
                              </MenuItem>
                              <MenuItem icon={<FiEdit />}>
                                Edit Contact
                              </MenuItem>
                              <MenuItem icon={<FiTrash2 />} color="red.500">
                                Delete Contact
                              </MenuItem>
                            </MenuList>
                          </Menu>
                        </Td>
                      </Tr>
                    ))}
                  </Tbody>
                </Table>
              </Box>
            )}
          </CardBody>
        </Card>

        {/* Pagination */}
        {totalPages > 1 && (
          <Flex justify="center" mb={6}>
            <HStack spacing={2}>
              <Button
                size="sm"
                onClick={() => setCurrentPage(Math.max(1, currentPage - 1))}
                isDisabled={currentPage === 1}
              >
                Previous
              </Button>
              
              {[...Array(totalPages)].map((_, i) => (
                <Button
                  key={i + 1}
                  size="sm"
                  variant={currentPage === i + 1 ? 'solid' : 'outline'}
                  colorScheme={currentPage === i + 1 ? 'blue' : 'gray'}
                  onClick={() => setCurrentPage(i + 1)}
                >
                  {i + 1}
                </Button>
              ))}
              
              <Button
                size="sm"
                onClick={() => setCurrentPage(Math.min(totalPages, currentPage + 1))}
                isDisabled={currentPage === totalPages}
              >
                Next
              </Button>
            </HStack>
          </Flex>
        )}

        {/* Quick Actions */}
        <Card>
          <CardHeader>
            <Heading size="md" color={primaryTextColor} display="flex" alignItems="center">
              <Icon as={FiBarChart} mr={2} color="var(--accent-color)" />
              Quick Actions
            </Heading>
          </CardHeader>
          <CardBody>
            <HStack spacing={4} flexWrap="wrap">
              <Button
                leftIcon={<FiBarChart />}
                colorScheme="blue"
                variant="outline"
                size="md"
              >
                Generate Report
              </Button>
              <Button
                leftIcon={<FiDownload />}
                colorScheme="green"
                variant="outline"
                size="md"
              >
                Export Data
              </Button>
              <Button
                leftIcon={<FiUpload />}
                colorScheme="purple"
                variant="outline"
                size="md"
              >
                Import Contacts
              </Button>
              <Button
                leftIcon={<FiSearch />}
                variant="ghost"
                size="md"
              >
                Advanced Search
              </Button>
            </HStack>
          </CardBody>
        </Card>
      </Box>

      {/* Add Contact Modal */}
      <Modal isOpen={isOpen} onClose={onClose} size="xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>Add New Contact</ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <Text>Add contact form would go here...</Text>
            {/* TODO: Implement contact form */}
          </ModalBody>
        </ModalContent>
      </Modal>
    </SimpleLayout>
  );
}
