'use client';

import React, { useState, useEffect } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useModulePermissions } from '@/hooks/usePermissions';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { contactService } from '@/services/contactService';
import { Contact } from '@/types/contact';
import ContactForm from '@/components/forms/ContactForm';
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
  IconButton,
  Tooltip,
  Divider,
} from '@chakra-ui/react';
import { 
  FiPlus, 
  FiEye, 
  FiEdit, 
  FiTrash2,
  FiSearch, 
  FiFilter,
  FiRefreshCw,
  FiPhone,
  FiMail,
  FiMapPin,
  FiUser,
  FiHome,
  FiUsers,
  FiChevronDown,
  FiChevronRight
} from 'react-icons/fi';

export default function GroupedTableContactsPage() {
  const { token } = useAuth();
  const { canCreate, canEdit, canDelete } = useModulePermissions('contacts');
  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();
  const { isOpen: isViewOpen, onOpen: onViewOpen, onClose: onViewClose } = useDisclosure();
  
  // State management
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedContact, setSelectedContact] = useState<Contact | null>(null);
  const [viewContact, setViewContact] = useState<Contact | null>(null);
  const [formLoading, setFormLoading] = useState(false);
  const [deleteLoading, setDeleteLoading] = useState<number | null>(null);
  
  // Group expansion states
  const [expandedGroups, setExpandedGroups] = useState<{[key: string]: boolean}>({
    CUSTOMER: true,
    EMPLOYEE: true,
    VENDOR: true,
  });
  
  // Theme-aware colors
  const headingColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const subheadingColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const tableBg = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const primaryTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const hoverBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');
  const groupHeaderBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');

  // Load contacts from backend
  const loadContacts = async () => {
    try {
      setLoading(true);
      const authToken = token;
      
      console.log('Loading contacts... Token available:', !!authToken);
      
      if (!authToken) {
        return;
      }
      
      // Get contacts using the authenticated endpoint
      const contactsData = await contactService.getContacts(authToken);
      
      // Apply search filter on frontend for better performance
      if (searchQuery && contactsData.length > 0) {
        const filtered = contactsData.filter(contact =>
          contact.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (contact.email && contact.email.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.phone && contact.phone.includes(searchQuery)) ||
          (contact.mobile && contact.mobile.includes(searchQuery)) ||
          (contact.code && contact.code.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.pic_name && contact.pic_name.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.external_id && contact.external_id.toLowerCase().includes(searchQuery.toLowerCase()))
        );
        setContacts(filtered || []);
      } else {
        setContacts(contactsData || []);
      }
      
    } catch (error) {
      console.error('Error loading contacts:', error);
      setContacts([]);
      
      const errorMessage = error instanceof Error ? error.message : 'Failed to load contacts';
      toast({
        title: 'Error Loading Contacts',
        description: errorMessage,
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
  }, [searchQuery, token]);

  // Group contacts by type
  const groupedContacts = {
    CUSTOMER: contacts.filter(c => c.type === 'CUSTOMER'),
    EMPLOYEE: contacts.filter(c => c.type === 'EMPLOYEE'),
    VENDOR: contacts.filter(c => c.type === 'VENDOR'),
  };

  // Calculate statistics
  const stats = {
    total: contacts.length,
    active: contacts.filter(c => c.is_active).length,
    customers: groupedContacts.CUSTOMER.length,
    employees: groupedContacts.EMPLOYEE.length,
    vendors: groupedContacts.VENDOR.length,
  };

  // Toggle group expansion
  const toggleGroupExpansion = (groupType: string) => {
    setExpandedGroups(prev => ({
      ...prev,
      [groupType]: !prev[groupType]
    }));
  };

  // Get group configuration
  const getGroupConfig = (type: string) => {
    switch (type) {
      case 'CUSTOMER':
        return {
          label: 'CUSTOMERS',
          icon: FiUser,
          color: 'blue',
          bgColor: 'blue.50',
          textColor: 'blue.600'
        };
      case 'EMPLOYEE':
        return {
          label: 'EMPLOYEES',
          icon: FiUsers,
          color: 'green',
          bgColor: 'green.50',
          textColor: 'green.600'
        };
      case 'VENDOR':
        return {
          label: 'VENDORS',
          icon: FiHome,
          color: 'purple',
          bgColor: 'purple.50',
          textColor: 'purple.600'
        };
      default:
        return {
          label: type,
          icon: FiUser,
          color: 'gray',
          bgColor: 'gray.50',
          textColor: 'gray.600'
        };
    }
  };

  // Handle contact actions
  const handleCreateContact = async (contactData: Partial<Contact>) => {
    const authToken = token;
    
    if (!authToken) {
      toast({
        title: 'Authentication Error',
        description: 'Authentication token is required to create contacts',
        status: 'error',
        duration: 5000,
      });
      return;
    }
    
    setFormLoading(true);
    try {
      await contactService.createContact(authToken, contactData);
      toast({
        title: 'Success',
        description: 'Contact created successfully',
        status: 'success',
        duration: 3000,
      });
      onClose();
      setSelectedContact(null);
      await loadContacts();
    } catch (error) {
      console.error('Error creating contact:', error);
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to create contact',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setFormLoading(false);
    }
  };

  const handleUpdateContact = async (contactData: Partial<Contact>) => {
    const authToken = token;
    
    if (!authToken) {
      toast({
        title: 'Authentication Error',
        description: 'Authentication token is required to update contacts',
        status: 'error',
        duration: 5000,
      });
      return;
    }
    
    if (!selectedContact) return;

    setFormLoading(true);
    try {
      await contactService.updateContact(authToken, selectedContact.id.toString(), contactData);
      toast({
        title: 'Success',
        description: 'Contact updated successfully',
        status: 'success',
        duration: 3000,
      });
      onClose();
      setSelectedContact(null);
      await loadContacts();
    } catch (error) {
      console.error('Error updating contact:', error);
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to update contact',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setFormLoading(false);
    }
  };

  const handleDeleteContact = async (contact: Contact) => {
    if (!confirm(`Are you sure you want to delete ${contact.name}?`)) return;

    const authToken = token;
    
    if (!authToken) {
      toast({
        title: 'Authentication Error',
        description: 'Authentication token is required to delete contacts',
        status: 'error',
        duration: 5000,
      });
      return;
    }

    setDeleteLoading(contact.id);
    try {
      await contactService.deleteContact(authToken, contact.id.toString());
      toast({
        title: 'Success',
        description: 'Contact deleted successfully',
        status: 'success',
        duration: 3000,
      });
      await loadContacts();
    } catch (error) {
      console.error('Error deleting contact:', error);
      toast({
        title: 'Error',
        description: error instanceof Error ? error.message : 'Failed to delete contact',
        status: 'error',
        duration: 5000,
      });
    } finally {
      setDeleteLoading(null);
    }
  };


  const handleViewContact = (contact: Contact) => {
    setViewContact(contact);
    onViewOpen();
  };

  const handleEditContact = (contact: Contact) => {
    setSelectedContact(contact);
    onOpen();
  };

  const handleAddContact = () => {
    setSelectedContact(null);
    onOpen();
  };

  const handleModalClose = () => {
    onClose();
    setSelectedContact(null);
  };

  const handleViewModalClose = () => {
    onViewClose();
    setViewContact(null);
  };

  // Render contact group table
  const renderContactGroup = (groupType: string, contactList: Contact[]) => {
    const config = getGroupConfig(groupType);
    const isExpanded = expandedGroups[groupType];

    return (
      <Card key={groupType} mb={4} variant="outline">
        {/* Group Header */}
        <CardHeader 
          py={3} 
          bg={config.bgColor}
          cursor="pointer"
          onClick={() => toggleGroupExpansion(groupType)}
          _hover={{ opacity: 0.8 }}
        >
          <Flex justify="space-between" align="center">
            <HStack spacing={3}>
              <Icon 
                as={isExpanded ? FiChevronDown : FiChevronRight} 
                color={config.textColor}
                fontSize="lg"
              />
              <Icon as={config.icon} color={config.textColor} fontSize="lg" />
              <Text 
                fontSize="sm" 
                fontWeight="bold" 
                color={config.textColor}
                textTransform="uppercase"
                letterSpacing="wider"
              >
                {config.label}
              </Text>
              <Badge 
                colorScheme={config.color}
                variant="solid"
                borderRadius="full"
                px={2}
                py={1}
                fontSize="xs"
              >
                {contactList.length} item{contactList.length !== 1 ? 's' : ''}
              </Badge>
            </HStack>
          </Flex>
        </CardHeader>

        {/* Group Content */}
        {isExpanded && (
          <CardBody p={0}>
            {contactList.length === 0 ? (
              <Box p={8} textAlign="center">
                <Text color={textColor} fontSize="sm">
                  No {config.label.toLowerCase()} found
                </Text>
              </Box>
            ) : (
              <Table size="sm" variant="simple">
                <Thead bg={groupHeaderBg}>
                  <Tr>
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">NAME</Th>
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">EXTERNAL ID</Th>
                    {groupType !== 'EMPLOYEE' && (
                      <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">PIC NAME</Th>
                    )}
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">EMAIL</Th>
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">PHONE</Th>
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">ADDRESS</Th>
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold">STATUS</Th>
                    <Th borderColor={borderColor} fontSize="xs" color={textColor} fontWeight="bold" textAlign="center">ACTIONS</Th>
                  </Tr>
                </Thead>
                <Tbody>
                  {contactList.map((contact, index) => (
                    <Tr 
                      key={contact.id} 
                      _hover={{ bg: hoverBg }} 
                      transition="all 0.2s ease"
                      borderBottom={index === contactList.length - 1 ? 'none' : '1px solid'}
                      borderColor={borderColor}
                    >
                      <Td borderColor={borderColor} py={3}>
                        <Text fontWeight="medium" color={primaryTextColor} fontSize="sm">
                          {contact.name}
                        </Text>
                        {contact.code && (
                          <Text fontSize="xs" color={textColor} fontFamily="monospace">
                            {contact.code}
                          </Text>
                        )}
                      </Td>
                      <Td borderColor={borderColor} py={3}>
                        <Text fontSize="sm" color={textColor}>
                          {contact.external_id || '-'}
                        </Text>
                      </Td>
                      {groupType !== 'EMPLOYEE' && (
                        <Td borderColor={borderColor} py={3}>
                          <Text fontSize="sm" color={textColor}>
                            {contact.pic_name || '-'}
                          </Text>
                        </Td>
                      )}
                      <Td borderColor={borderColor} py={3}>
                        {contact.email ? (
                          <Text fontSize="sm" color="var(--accent-color)" _hover={{ textDecoration: 'underline' }}>
                            {contact.email}
                          </Text>
                        ) : (
                          <Text fontSize="sm" color={textColor}>-</Text>
                        )}
                      </Td>
                      <Td borderColor={borderColor} py={3}>
                        <Text fontSize="sm" color={textColor}>
                          {contact.phone || '-'}
                        </Text>
                      </Td>
                      <Td borderColor={borderColor} py={3}>
                        <Text fontSize="sm" color={textColor}>
                          {contact.address || '-'}
                        </Text>
                      </Td>
                      <Td borderColor={borderColor} py={3}>
                        <Badge 
                          colorScheme={contact.is_active ? 'green' : 'gray'}
                          variant="subtle"
                          fontSize="xs"
                          px={2}
                          py={1}
                          borderRadius="md"
                        >
                          {contact.is_active ? 'ACTIVE' : 'INACTIVE'}
                        </Badge>
                      </Td>
                      <Td borderColor={borderColor} py={3} textAlign="center">
                        <HStack spacing={1} justify="center">
                          <Tooltip label="View Details">
                            <IconButton
                              aria-label="View contact"
                              icon={<FiEye />}
                              size="sm"
                              variant="ghost"
                              colorScheme="blue"
                              onClick={() => handleViewContact(contact)}
                            />
                          </Tooltip>
                          {canEdit && (
                            <Tooltip label="Edit Contact">
                              <IconButton
                                aria-label="Edit contact"
                                icon={<FiEdit />}
                                size="sm"
                                variant="ghost"
                                colorScheme="green"
                                onClick={() => handleEditContact(contact)}
                              />
                            </Tooltip>
                          )}
                          {canDelete && (
                            <Tooltip label="Delete Contact">
                              <IconButton
                                aria-label="Delete contact"
                                icon={<FiTrash2 />}
                                size="sm"
                                variant="ghost"
                                colorScheme="red"
                                onClick={() => handleDeleteContact(contact)}
                                isLoading={deleteLoading === contact.id}
                              />
                            </Tooltip>
                          )}
                        </HStack>
                      </Td>
                    </Tr>
                  ))}
                </Tbody>
              </Table>
            )}
          </CardBody>
        )}
      </Card>
    );
  };

  return (
    <SimpleLayout>
      <Box>
        {/* Header */}
        <Flex justify="space-between" align="center" mb={6} wrap="wrap" gap={4}>
          <VStack align="start" spacing={1}>
            <Heading size="xl" color={headingColor} fontWeight="600">
              Contacts
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
            
            
            {canCreate && (
              <Button
                colorScheme="blue"
                leftIcon={<FiPlus />}
                size="md"
                px={6}
                fontWeight="medium"
                onClick={handleAddContact}
                _hover={{ 
                  transform: 'translateY(-1px)',
                  boxShadow: 'lg'
                }}
              >
                Add New Contact
              </Button>
            )}
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
                  Employees
                </StatLabel>
                <HStack>
                  <Icon as={FiUsers} color="green.500" />
                  <StatNumber color="green.500" fontSize="2xl" fontWeight="bold">
                    {stats.employees}
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
                  <Icon as={FiHome} color="purple.500" />
                  <StatNumber color="purple.500" fontSize="2xl" fontWeight="bold">
                    {stats.vendors}
                  </StatNumber>
                </HStack>
              </Stat>
            </CardBody>
          </Card>
        </SimpleGrid>

        {/* Search Filter */}
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
                    placeholder="Search by name, email, phone, external ID..."
                    value={searchQuery}
                    onChange={(e) => setSearchQuery(e.target.value)}
                    bg={tableBg}
                  />
                </InputGroup>
              </Box>
              
              <Button
                leftIcon={<FiFilter />}
                variant="outline"
                onClick={() => setSearchQuery('')}
              >
                Clear Search
              </Button>
            </Flex>
          </CardBody>
        </Card>

        {/* Grouped Contact Tables */}
        {loading ? (
          <Card>
            <CardBody>
              <Flex justify="center" align="center" py={10}>
                <Spinner size="lg" color="var(--accent-color)" />
                <Text ml={4} color={textColor}>Loading contacts...</Text>
              </Flex>
            </CardBody>
          </Card>
        ) : contacts.length === 0 ? (
          <Card>
            <CardBody>
              <Alert status="info" variant="subtle">
                <AlertIcon />
                <Box>
                  <Text fontWeight="medium">No contacts found</Text>
                  <Text fontSize="sm" color={textColor}>
                    Try adjusting your search criteria or add a new contact.
                  </Text>
                </Box>
              </Alert>
            </CardBody>
          </Card>
        ) : (
          <VStack spacing={4} align="stretch">
            {renderContactGroup('CUSTOMER', groupedContacts.CUSTOMER)}
            {renderContactGroup('EMPLOYEE', groupedContacts.EMPLOYEE)}
            {renderContactGroup('VENDOR', groupedContacts.VENDOR)}
          </VStack>
        )}
      </Box>

      {/* Contact Form Modal */}
      <Modal isOpen={isOpen} onClose={handleModalClose} size="6xl" scrollBehavior="inside">
        <ModalOverlay />
        <ModalContent maxH="90vh">
          <ModalHeader>
            {selectedContact ? 'Edit Contact' : 'Add New Contact'}
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <ContactForm
              contact={selectedContact}
              onSubmit={selectedContact ? handleUpdateContact : handleCreateContact}
              onCancel={handleModalClose}
              isLoading={formLoading}
            />
          </ModalBody>
        </ModalContent>
      </Modal>

      {/* View Contact Detail Modal */}
      <Modal isOpen={isViewOpen} onClose={handleViewModalClose} size="4xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            <HStack>
              <Icon as={FiEye} color="blue.500" />
              <Text>Contact Details</Text>
            </HStack>
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            {viewContact && (
              <VStack spacing={6} align="stretch">
                {/* Contact Header */}
                <Box>
                  <HStack justify="space-between" align="start" mb={4}>
                    <VStack align="start" spacing={1}>
                      <Heading size="lg" color={primaryTextColor}>
                        {viewContact.name}
                      </Heading>
                      {viewContact.code && (
                        <Text fontSize="sm" color={textColor} fontFamily="monospace">
                          Code: {viewContact.code}
                        </Text>
                      )}
                    </VStack>
                    <Badge 
                      colorScheme={viewContact.is_active ? 'green' : 'gray'}
                      variant="solid"
                      px={3}
                      py={1}
                      borderRadius="md"
                    >
                      {viewContact.is_active ? 'ACTIVE' : 'INACTIVE'}
                    </Badge>
                  </HStack>
                  
                  <Badge 
                    colorScheme={getGroupConfig(viewContact.type).color}
                    variant="outline"
                    px={2}
                    py={1}
                  >
                    {viewContact.type}
                  </Badge>
                </Box>

                <Divider />

                {/* Contact Information */}
                <SimpleGrid columns={{ base: 1, md: 2 }} spacing={6}>
                  {/* Basic Information */}
                  <VStack align="start" spacing={4}>
                    <Heading size="md" color={primaryTextColor}>Basic Information</Heading>
                    
                    {viewContact.external_id && (
                      <Box>
                        <Text fontSize="sm" fontWeight="medium" color={textColor} mb={1}>
                          External ID
                        </Text>
                        <Text fontSize="md" color={primaryTextColor}>
                          {viewContact.external_id}
                        </Text>
                      </Box>
                    )}
                    
                    {viewContact.pic_name && (
                      <Box>
                        <Text fontSize="sm" fontWeight="medium" color={textColor} mb={1}>
                          PIC Name
                        </Text>
                        <Text fontSize="md" color={primaryTextColor}>
                          {viewContact.pic_name}
                        </Text>
                      </Box>
                    )}
                    
                    {viewContact.description && (
                      <Box>
                        <Text fontSize="sm" fontWeight="medium" color={textColor} mb={1}>
                          Description
                        </Text>
                        <Text fontSize="md" color={primaryTextColor}>
                          {viewContact.description}
                        </Text>
                      </Box>
                    )}
                  </VStack>

                  {/* Contact Information */}
                  <VStack align="start" spacing={4}>
                    <Heading size="md" color={primaryTextColor}>Contact Information</Heading>
                    
                    {viewContact.email && (
                      <Box>
                        <HStack mb={1}>
                          <Icon as={FiMail} color="blue.500" />
                          <Text fontSize="sm" fontWeight="medium" color={textColor}>
                            Email
                          </Text>
                        </HStack>
                        <Text fontSize="md" color="blue.500">
                          {viewContact.email}
                        </Text>
                      </Box>
                    )}
                    
                    {viewContact.phone && (
                      <Box>
                        <HStack mb={1}>
                          <Icon as={FiPhone} color="green.500" />
                          <Text fontSize="sm" fontWeight="medium" color={textColor}>
                            Phone
                          </Text>
                        </HStack>
                        <Text fontSize="md" color={primaryTextColor}>
                          {viewContact.phone}
                        </Text>
                      </Box>
                    )}
                    
                    {viewContact.mobile && (
                      <Box>
                        <HStack mb={1}>
                          <Icon as={FiPhone} color="green.500" />
                          <Text fontSize="sm" fontWeight="medium" color={textColor}>
                            Mobile
                          </Text>
                        </HStack>
                        <Text fontSize="md" color={primaryTextColor}>
                          {viewContact.mobile}
                        </Text>
                      </Box>
                    )}
                    
                    {viewContact.address && (
                      <Box>
                        <HStack mb={1}>
                          <Icon as={FiMapPin} color="purple.500" />
                          <Text fontSize="sm" fontWeight="medium" color={textColor}>
                            Address
                          </Text>
                        </HStack>
                        <Text fontSize="md" color={primaryTextColor}>
                          {viewContact.address}
                        </Text>
                      </Box>
                    )}
                  </VStack>
                </SimpleGrid>

                {/* Action Buttons */}
                <Divider />
                <HStack justify="end" spacing={3}>
                  <Button
                    leftIcon={<FiEdit />}
                    colorScheme="blue"
                    variant="outline"
                    onClick={() => {
                      handleViewModalClose();
                      handleEditContact(viewContact);
                    }}
                  >
                    Edit Contact
                  </Button>
                  <Button
                    leftIcon={<FiTrash2 />}
                    colorScheme="red"
                    variant="outline"
                    onClick={() => {
                      handleViewModalClose();
                      handleDeleteContact(viewContact);
                    }}
                  >
                    Delete Contact
                  </Button>
                </HStack>
              </VStack>
            )}
          </ModalBody>
        </ModalContent>
      </Modal>
    </SimpleLayout>
  );
}
