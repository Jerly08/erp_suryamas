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
  useColorMode,
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
  Avatar,
  Divider,
  Stack,
  Tag,
  TagLabel,
  Wrap,
  WrapItem,
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
  FiUsers,
  FiGrid,
  FiList,
  FiCalendar,
  FiStar
} from 'react-icons/fi';

interface ContactsPageProps {}

export default function CardGridContactsPage({}: ContactsPageProps) {
  const { user, token } = useAuth();
  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();
  
  // State management
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [filterType, setFilterType] = useState('ALL');
  const [viewMode, setViewMode] = useState<'grid' | 'list'>('grid');
  const [sortBy, setSortBy] = useState('name');
  
  // Theme-aware colors
  const headingColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const subheadingColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const cardBg = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const primaryTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const hoverBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');

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

      // Apply sorting
      contactsData.sort((a, b) => {
        switch (sortBy) {
          case 'name':
            return a.name.localeCompare(b.name);
          case 'type':
            return a.type.localeCompare(b.type);
          case 'created_at':
            return new Date(b.created_at).getTime() - new Date(a.created_at).getTime();
          default:
            return 0;
        }
      });
      
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
  }, [filterType, searchQuery, sortBy, token]);

  // Calculate statistics
  const stats = {
    total: contacts.length,
    active: contacts.filter(c => c.is_active).length,
    customers: contacts.filter(c => c.type === 'CUSTOMER').length,
    vendors: contacts.filter(c => c.type === 'VENDOR').length,
    employees: contacts.filter(c => c.type === 'EMPLOYEE').length,
  };

  // Get contact type badge color and icon
  const getTypeColor = (type: string) => {
    switch (type) {
      case 'CUSTOMER': return 'blue';
      case 'VENDOR': return 'green';
      case 'EMPLOYEE': return 'purple';
      default: return 'gray';
    }
  };

  const getTypeIcon = (type: string) => {
    switch (type) {
      case 'CUSTOMER': return FiUser;
      case 'VENDOR': return FiHome;
      case 'EMPLOYEE': return FiUsers;
      default: return FiUser;
    }
  };

  // Generate avatar initials
  const getInitials = (name: string) => {
    return name
      .split(' ')
      .map(word => word[0])
      .join('')
      .toUpperCase()
      .slice(0, 2);
  };

  // Contact Card Component
  const ContactCard = ({ contact }: { contact: Contact }) => (
    <Card 
      bg={cardBg} 
      borderColor={borderColor}
      _hover={{ 
        transform: 'translateY(-2px)',
        boxShadow: 'lg',
        borderColor: 'var(--accent-color)'
      }}
      transition="all 0.2s ease"
      cursor="pointer"
    >
      <CardBody p={6}>
        {/* Header with Avatar and Type */}
        <Flex align="start" justify="space-between" mb={4}>
          <HStack spacing={3}>
            <Avatar 
              size="md" 
              name={contact.name}
              bg={`${getTypeColor(contact.type)}.500`}
              color="white"
              fontWeight="bold"
            >
              {getInitials(contact.name)}
            </Avatar>
            <VStack align="start" spacing={0}>
              <Text fontWeight="bold" color={primaryTextColor} fontSize="lg">
                {contact.name}
              </Text>
              <HStack>
                <Icon as={getTypeIcon(contact.type)} color={`${getTypeColor(contact.type)}.500`} size="14px" />
                <Badge 
                  colorScheme={getTypeColor(contact.type)}
                  variant="subtle"
                  fontSize="xs"
                >
                  {contact.type}
                </Badge>
              </HStack>
            </VStack>
          </HStack>
          
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
        </Flex>

        {/* Contact Code */}
        <Box mb={4}>
          <Text fontSize="xs" color={textColor} textTransform="uppercase" letterSpacing="wider" mb={1}>
            Contact Code
          </Text>
          <Text fontFamily="monospace" color={primaryTextColor} fontWeight="medium">
            {contact.code}
          </Text>
        </Box>

        <Divider mb={4} />

        {/* Contact Information */}
        <VStack align="stretch" spacing={3}>
          {contact.email && (
            <HStack>
              <Icon as={FiMail} color={textColor} size="16px" />
              <Text fontSize="sm" color="var(--accent-color)" _hover={{ textDecoration: 'underline' }}>
                {contact.email}
              </Text>
            </HStack>
          )}
          
          {contact.phone && (
            <HStack>
              <Icon as={FiPhone} color={textColor} size="16px" />
              <Text fontSize="sm" color={textColor}>
                {contact.phone}
              </Text>
            </HStack>
          )}
          
          {contact.pic_name && (
            <HStack>
              <Icon as={FiUser} color={textColor} size="16px" />
              <VStack align="start" spacing={0}>
                <Text fontSize="xs" color={textColor} textTransform="uppercase">
                  Person in Charge
                </Text>
                <Text fontSize="sm" color={primaryTextColor} fontWeight="medium">
                  {contact.pic_name}
                </Text>
              </VStack>
            </HStack>
          )}
          
          {contact.address && (
            <HStack align="start">
              <Icon as={FiMapPin} color={textColor} size="16px" mt={1} />
              <Text fontSize="sm" color={textColor} noOfLines={2}>
                {contact.address}
              </Text>
            </HStack>
          )}
        </VStack>

        <Divider my={4} />

        {/* Footer with Status and External ID */}
        <Flex justify="space-between" align="center">
          <Badge 
            colorScheme={contact.is_active ? 'green' : 'gray'}
            variant="solid"
            px={3}
            py={1}
            borderRadius="full"
          >
            {contact.is_active ? 'ACTIVE' : 'INACTIVE'}
          </Badge>
          
          {contact.external_id && (
            <Text fontSize="xs" color={textColor} fontFamily="monospace">
              {contact.external_id}
            </Text>
          )}
        </Flex>
      </CardBody>
    </Card>
  );

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
            <HStack spacing={1} bg={cardBg} border="1px solid" borderColor={borderColor} borderRadius="md" p={1}>
              <IconButton
                aria-label="Grid view"
                icon={<FiGrid />}
                size="sm"
                variant={viewMode === 'grid' ? 'solid' : 'ghost'}
                colorScheme={viewMode === 'grid' ? 'blue' : 'gray'}
                onClick={() => setViewMode('grid')}
              />
              <IconButton
                aria-label="List view"
                icon={<FiList />}
                size="sm"
                variant={viewMode === 'list' ? 'solid' : 'ghost'}
                colorScheme={viewMode === 'list' ? 'blue' : 'gray'}
                onClick={() => setViewMode('list')}
              />
            </HStack>
            
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

        {/* Statistics Dashboard */}
        <SimpleGrid columns={{ base: 1, md: 2, lg: 5 }} spacing={4} mb={6}>
          <Card bg="linear-gradient(135deg, var(--accent-color), #4dabf7)" color="white" boxShadow="xl">
            <CardBody>
              <Stat>
                <HStack>
                  <Icon as={FiUsers} boxSize={6} />
                  <Box>
                    <StatLabel fontSize="sm" opacity={0.9}>
                      Total Contacts
                    </StatLabel>
                    <StatNumber fontSize="3xl" fontWeight="bold">
                      {stats.total}
                    </StatNumber>
                  </Box>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
          
          <Card bg="linear-gradient(135deg, var(--success-color), #51cf66)" color="white" boxShadow="xl">
            <CardBody>
              <Stat>
                <HStack>
                  <Icon as={FiUser} boxSize={6} />
                  <Box>
                    <StatLabel fontSize="sm" opacity={0.9}>
                      Active
                    </StatLabel>
                    <StatNumber fontSize="3xl" fontWeight="bold">
                      {stats.active}
                    </StatNumber>
                  </Box>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
          
          <Card bg="linear-gradient(135deg, #4299E1, #63B3ED)" color="white" boxShadow="xl">
            <CardBody>
              <Stat>
                <HStack>
                  <Icon as={FiUser} boxSize={6} />
                  <Box>
                    <StatLabel fontSize="sm" opacity={0.9}>
                      Customers
                    </StatLabel>
                    <StatNumber fontSize="3xl" fontWeight="bold">
                      {stats.customers}
                    </StatNumber>
                  </Box>
                </HStack>
              </Stat>
            </CardBody>
          </Card>

          <Card bg="linear-gradient(135deg, #48BB78, #68D391)" color="white" boxShadow="xl">
            <CardBody>
              <Stat>
                <HStack>
                  <Icon as={FiHome} boxSize={6} />
                  <Box>
                    <StatLabel fontSize="sm" opacity={0.9}>
                      Vendors
                    </StatLabel>
                    <StatNumber fontSize="3xl" fontWeight="bold">
                      {stats.vendors}
                    </StatNumber>
                  </Box>
                </HStack>
              </Stat>
            </CardBody>
          </Card>

          <Card bg="linear-gradient(135deg, #9F7AEA, #B794F6)" color="white" boxShadow="xl">
            <CardBody>
              <Stat>
                <HStack>
                  <Icon as={FiUsers} boxSize={6} />
                  <Box>
                    <StatLabel fontSize="sm" opacity={0.9}>
                      Employees
                    </StatLabel>
                    <StatNumber fontSize="3xl" fontWeight="bold">
                      {stats.employees}
                    </StatNumber>
                  </Box>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </SimpleGrid>

        {/* Filters and Controls */}
        <Card mb={6}>
          <CardBody>
            <Flex gap={4} align="end" wrap="wrap">
              <Box flex="1" minW="300px">
                <Text fontSize="sm" fontWeight="medium" mb={2} color={textColor}>
                  Search Contacts
                </Text>
                <InputGroup size="lg">
                  <InputLeftElement pointerEvents="none">
                    <FiSearch color={textColor} />
                  </InputLeftElement>
                  <Input
                    placeholder="Search by name, email, phone, or PIC..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    bg={cardBg}
                    border="2px solid"
                    borderColor={borderColor}
                    _focus={{
                      borderColor: 'var(--accent-color)',
                      boxShadow: '0 0 0 1px var(--accent-color)'
                    }}
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
                  bg={cardBg}
                  size="lg"
                >
                  <option value="ALL">All Types</option>
                  <option value="CUSTOMER">Customers</option>
                  <option value="VENDOR">Vendors</option>
                  <option value="EMPLOYEE">Employees</option>
                </Select>
              </Box>

              <Box minW="160px">
                <Text fontSize="sm" fontWeight="medium" mb={2} color={textColor}>
                  Sort By
                </Text>
                <Select
                  value={sortBy}
                  onChange={(e) => setSortBy(e.target.value)}
                  bg={cardBg}
                  size="lg"
                >
                  <option value="name">Name</option>
                  <option value="type">Type</option>
                  <option value="created_at">Date Added</option>
                </Select>
              </Box>
              
              <Button
                leftIcon={<FiFilter />}
                variant="outline"
                size="lg"
                onClick={() => {
                  setSearchQuery('');
                  setFilterType('ALL');
                  setSortBy('name');
                }}
              >
                Clear All
              </Button>
            </Flex>
          </CardBody>
        </Card>

        {/* Contacts Content */}
        <Box mb={6}>
          {loading ? (
            <Flex justify="center" align="center" py={20}>
              <VStack spacing={4}>
                <Spinner size="xl" color="var(--accent-color)" thickness="4px" />
                <Text color={textColor} fontSize="lg">Loading contacts...</Text>
              </VStack>
            </Flex>
          ) : contacts.length === 0 ? (
            <Card p={10}>
              <VStack spacing={6}>
                <Icon as={FiUsers} boxSize={20} color={textColor} />
                <VStack spacing={2}>
                  <Text fontSize="xl" fontWeight="bold" color={primaryTextColor}>
                    No contacts found
                  </Text>
                  <Text color={textColor} textAlign="center" maxW="400px">
                    {searchQuery || filterType !== 'ALL' 
                      ? 'Try adjusting your search criteria or filters.'
                      : 'Start by adding your first contact to the system.'
                    }
                  </Text>
                </VStack>
                <Button
                  leftIcon={<FiPlus />}
                  colorScheme="blue"
                  size="lg"
                  onClick={onOpen}
                >
                  Add Your First Contact
                </Button>
              </VStack>
            </Card>
          ) : (
            <>
              {/* Results Header */}
              <Flex justify="space-between" align="center" mb={4}>
                <Text color={textColor}>
                  Showing {contacts.length} contact{contacts.length !== 1 ? 's' : ''}
                  {searchQuery && ` matching "${searchQuery}"`}
                  {filterType !== 'ALL' && ` (${filterType.toLowerCase()})`}
                </Text>
              </Flex>

              {/* Grid View */}
              <SimpleGrid 
                columns={{ 
                  base: 1, 
                  md: viewMode === 'grid' ? 2 : 1, 
                  lg: viewMode === 'grid' ? 3 : 1,
                  xl: viewMode === 'grid' ? 4 : 1 
                }} 
                spacing={6}
              >
                {contacts.map((contact) => (
                  <ContactCard key={contact.id} contact={contact} />
                ))}
              </SimpleGrid>
            </>
          )}
        </Box>

        {/* Quick Actions */}
        <Card>
          <CardHeader>
            <Heading size="md" color={primaryTextColor} display="flex" alignItems="center">
              <Icon as={FiBarChart} mr={2} color="var(--accent-color)" />
              Quick Actions & Tools
            </Heading>
          </CardHeader>
          <CardBody>
            <Wrap spacing={4}>
              <WrapItem>
                <Button
                  leftIcon={<FiBarChart />}
                  colorScheme="blue"
                  variant="outline"
                  size="md"
                  _hover={{ bg: 'blue.50' }}
                >
                  Generate Report
                </Button>
              </WrapItem>
              <WrapItem>
                <Button
                  leftIcon={<FiDownload />}
                  colorScheme="green"
                  variant="outline"
                  size="md"
                  _hover={{ bg: 'green.50' }}
                >
                  Export Contacts
                </Button>
              </WrapItem>
              <WrapItem>
                <Button
                  leftIcon={<FiUpload />}
                  colorScheme="purple"
                  variant="outline"
                  size="md"
                  _hover={{ bg: 'purple.50' }}
                >
                  Import Contacts
                </Button>
              </WrapItem>
              <WrapItem>
                <Button
                  leftIcon={<FiSearch />}
                  variant="outline"
                  size="md"
                  _hover={{ bg: hoverBg }}
                >
                  Advanced Search
                </Button>
              </WrapItem>
              <WrapItem>
                <Button
                  leftIcon={<FiStar />}
                  colorScheme="orange"
                  variant="outline"
                  size="md"
                  _hover={{ bg: 'orange.50' }}
                >
                  Favorites
                </Button>
              </WrapItem>
            </Wrap>
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
