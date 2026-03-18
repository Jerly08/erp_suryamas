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
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Menu,
  MenuButton,
  MenuList,
  MenuItem,
  IconButton,
  Tooltip,
  Avatar,
  Divider,
  Progress,
  CircularProgress,
  CircularProgressLabel,
  Grid,
  GridItem,
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
  FiRefreshCw,
  FiMoreVertical,
  FiPhone,
  FiMail,
  FiUser,
  FiHome,
  FiUsers,
  FiActivity,
  FiPieChart
} from 'react-icons/fi';

export default function DashboardTabsContactsPage() {
  const { token } = useAuth();
  const toast = useToast();
  const { isOpen, onOpen, onClose } = useDisclosure();
  
  // State management
  const [contacts, setContacts] = useState<Contact[]>([]);
  const [loading, setLoading] = useState(true);
  const [searchQuery, setSearchQuery] = useState('');
  const [activeTab, setActiveTab] = useState(0);
  
  // Theme-aware colors
  const headingColor = useColorModeValue('gray.800', 'var(--text-primary)');
  const subheadingColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const cardBg = useColorModeValue('white', 'var(--bg-secondary)');
  const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
  const textColor = useColorModeValue('gray.600', 'var(--text-secondary)');
  const primaryTextColor = useColorModeValue('gray.800', 'var(--text-primary)');
  // Load contacts from backend
  const loadContacts = async () => {
    try {
      setLoading(true);
      const authToken = token || localStorage.getItem('token');
      if (!authToken) {
        throw new Error('Authentication token not found');
      }

      const contactsData = await contactService.getContacts(authToken);
      
      // Apply search filter
      if (searchQuery) {
        const filtered = contactsData.filter(contact =>
          contact.name.toLowerCase().includes(searchQuery.toLowerCase()) ||
          (contact.email && contact.email.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.phone && contact.phone.includes(searchQuery)) ||
          (contact.mobile && contact.mobile.includes(searchQuery)) ||
          (contact.code && contact.code.toLowerCase().includes(searchQuery.toLowerCase())) ||
          (contact.pic_name && contact.pic_name.toLowerCase().includes(searchQuery.toLowerCase()))
        );
        setContacts(filtered);
      } else {
        setContacts(contactsData);
      }
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
  }, [searchQuery]);

  // Calculate comprehensive statistics
  const stats = {
    total: contacts.length,
    active: contacts.filter(c => c.is_active).length,
    inactive: contacts.filter(c => !c.is_active).length,
    customers: contacts.filter(c => c.type === 'CUSTOMER'),
    vendors: contacts.filter(c => c.type === 'VENDOR'),
    employees: contacts.filter(c => c.type === 'EMPLOYEE'),
    withEmail: contacts.filter(c => c.email).length,
    withPhone: contacts.filter(c => c.phone).length,
    withAddress: contacts.filter(c => c.address).length,
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

  // Render contact list for each tab
  const renderContactList = (contactList: Contact[], emptyMessage: string) => (
    <Box>
      {contactList.length === 0 ? (
        <Alert status="info" variant="subtle">
          <AlertIcon />
          <Text>{emptyMessage}</Text>
        </Alert>
      ) : (
        <SimpleGrid columns={{ base: 1, lg: 2 }} spacing={4}>
          {contactList.map((contact) => (
            <Card key={contact.id} bg={cardBg} _hover={{ boxShadow: 'md' }}>
              <CardBody>
                <Flex align="start" gap={4}>
                  <Avatar 
                    size="md" 
                    name={contact.name}
                    bg={`${getTypeColor(contact.type)}.500`}
                    color="white"
                  />
                  
                  <Box flex="1">
                    <Flex justify="space-between" align="start" mb={2}>
                      <VStack align="start" spacing={1}>
                        <Text fontWeight="bold" color={primaryTextColor}>
                          {contact.name}
                        </Text>
                        <HStack>
                          <Badge 
                            colorScheme={getTypeColor(contact.type)}
                            variant="subtle"
                            fontSize="xs"
                          >
                            {contact.type}
                          </Badge>
                          <Text fontSize="xs" fontFamily="monospace" color={textColor}>
                            {contact.code}
                          </Text>
                        </HStack>
                      </VStack>
                      
                      <Menu>
                        <MenuButton
                          as={IconButton}
                          icon={<FiMoreVertical />}
                          variant="ghost"
                          size="sm"
                        />
                        <MenuList>
                          <MenuItem icon={<FiEye />}>View</MenuItem>
                          <MenuItem icon={<FiEdit />}>Edit</MenuItem>
                          <MenuItem icon={<FiTrash2 />} color="red.500">Delete</MenuItem>
                        </MenuList>
                      </Menu>
                    </Flex>
                    
                    <VStack align="start" spacing={1} fontSize="sm">
                      {contact.email && (
                        <HStack>
                          <Icon as={FiMail} color={textColor} size="12px" />
                          <Text color={textColor}>{contact.email}</Text>
                        </HStack>
                      )}
                      {contact.phone && (
                        <HStack>
                          <Icon as={FiPhone} color={textColor} size="12px" />
                          <Text color={textColor}>{contact.phone}</Text>
                        </HStack>
                      )}
                      {contact.pic_name && (
                        <HStack>
                          <Icon as={FiUser} color={textColor} size="12px" />
                          <Text color={textColor}>{contact.pic_name}</Text>
                        </HStack>
                      )}
                    </VStack>
                  </Box>
                </Flex>
              </CardBody>
            </Card>
          ))}
        </SimpleGrid>
      )}
    </Box>
  );

  return (
    <SimpleLayout allowedRoles={['admin', 'finance', 'employee', 'director']}>
      <Box>
        {/* Header */}
        <Flex justify="space-between" align="center" mb={6} wrap="wrap" gap={4}>
          <VStack align="start" spacing={1}>
            <Heading size="xl" color={headingColor} fontWeight="600">
              Contact Management Dashboard
            </Heading>
            <Text color={subheadingColor} fontSize="md">
              Comprehensive contact management with analytics
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

        {/* Analytics Overview */}
        <Grid templateColumns={{ base: '1fr', lg: 'repeat(4, 1fr)' }} gap={6} mb={6}>
          {/* Key Statistics */}
          <GridItem colSpan={{ base: 1, lg: 2 }}>
            <Card>
              <CardHeader>
                <Heading size="md" color={primaryTextColor}>
                  <Icon as={FiBarChart} mr={2} color="var(--accent-color)" />
                  Contact Statistics
                </Heading>
              </CardHeader>
              <CardBody>
                <SimpleGrid columns={2} spacing={4}>
                  <VStack>
                    <CircularProgress value={(stats.active / stats.total) * 100} color="green.500" size="80px">
                      <CircularProgressLabel fontSize="sm" fontWeight="bold">
                        {Math.round((stats.active / stats.total) * 100)}%
                      </CircularProgressLabel>
                    </CircularProgress>
                    <Text fontSize="sm" color={textColor} textAlign="center">
                      Active Rate
                    </Text>
                  </VStack>
                  
                  <VStack align="stretch" spacing={3}>
                    <Box>
                      <Flex justify="space-between" mb={1}>
                        <Text fontSize="sm" color={textColor}>Email Coverage</Text>
                        <Text fontSize="sm" color={primaryTextColor}>
                          {stats.withEmail}/{stats.total}
                        </Text>
                      </Flex>
                      <Progress value={(stats.withEmail / stats.total) * 100} colorScheme="blue" size="sm" />
                    </Box>
                    
                    <Box>
                      <Flex justify="space-between" mb={1}>
                        <Text fontSize="sm" color={textColor}>Phone Coverage</Text>
                        <Text fontSize="sm" color={primaryTextColor}>
                          {stats.withPhone}/{stats.total}
                        </Text>
                      </Flex>
                      <Progress value={(stats.withPhone / stats.total) * 100} colorScheme="green" size="sm" />
                    </Box>
                    
                    <Box>
                      <Flex justify="space-between" mb={1}>
                        <Text fontSize="sm" color={textColor}>Address Coverage</Text>
                        <Text fontSize="sm" color={primaryTextColor}>
                          {stats.withAddress}/{stats.total}
                        </Text>
                      </Flex>
                      <Progress value={(stats.withAddress / stats.total) * 100} colorScheme="purple" size="sm" />
                    </Box>
                  </VStack>
                </SimpleGrid>
              </CardBody>
            </Card>
          </GridItem>

          {/* Type Distribution */}
          <GridItem>
            <Card h="full">
              <CardHeader>
                <Heading size="sm" color={primaryTextColor}>
                  <Icon as={FiPieChart} mr={2} color="var(--accent-color)" />
                  By Type
                </Heading>
              </CardHeader>
              <CardBody>
                <VStack spacing={4} align="stretch">
                  <HStack justify="space-between">
                    <HStack>
                      <Icon as={FiUser} color="blue.500" />
                      <Text fontSize="sm" color={textColor}>Customers</Text>
                    </HStack>
                    <Badge colorScheme="blue" variant="subtle">
                      {stats.customers.length}
                    </Badge>
                  </HStack>
                  
                  <HStack justify="space-between">
                    <HStack>
                      <Icon as={FiHome} color="green.500" />
                      <Text fontSize="sm" color={textColor}>Vendors</Text>
                    </HStack>
                    <Badge colorScheme="green" variant="subtle">
                      {stats.vendors.length}
                    </Badge>
                  </HStack>
                  
                  <HStack justify="space-between">
                    <HStack>
                      <Icon as={FiUsers} color="purple.500" />
                      <Text fontSize="sm" color={textColor}>Employees</Text>
                    </HStack>
                    <Badge colorScheme="purple" variant="subtle">
                      {stats.employees.length}
                    </Badge>
                  </HStack>
                </VStack>
              </CardBody>
            </Card>
          </GridItem>

          {/* Quick Summary */}
          <GridItem>
            <Card h="full">
              <CardHeader>
                <Heading size="sm" color={primaryTextColor}>
                  <Icon as={FiActivity} mr={2} color="var(--accent-color)" />
                  Quick Summary
                </Heading>
              </CardHeader>
              <CardBody>
                <VStack spacing={4} align="stretch">
                  <Box textAlign="center">
                    <Text fontSize="2xl" fontWeight="bold" color="var(--accent-color)">
                      {stats.total}
                    </Text>
                    <Text fontSize="sm" color={textColor}>Total Contacts</Text>
                  </Box>
                  
                  <Divider />
                  
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={textColor}>Active</Text>
                    <Text fontSize="sm" fontWeight="bold" color="green.500">
                      {stats.active}
                    </Text>
                  </HStack>
                  
                  <HStack justify="space-between">
                    <Text fontSize="sm" color={textColor}>Inactive</Text>
                    <Text fontSize="sm" fontWeight="bold" color="red.500">
                      {stats.inactive}
                    </Text>
                  </HStack>
                </VStack>
              </CardBody>
            </Card>
          </GridItem>
        </Grid>

        {/* Search Bar */}
        <Card mb={6}>
          <CardBody>
            <InputGroup size="lg">
              <InputLeftElement pointerEvents="none">
                <FiSearch color={textColor} />
              </InputLeftElement>
              <Input
                placeholder="Search contacts by name, email, phone, or person in charge..."
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
          </CardBody>
        </Card>

        {/* Tabs for Different Views */}
        <Card>
          <Tabs index={activeTab} onChange={setActiveTab} variant="enclosed" colorScheme="blue">
            <CardHeader pb={0}>
              <TabList>
                <Tab>
                  <Icon as={FiUsers} mr={2} />
                  All Contacts ({stats.total})
                </Tab>
                <Tab>
                  <Icon as={FiUser} mr={2} />
                  Customers ({stats.customers.length})
                </Tab>
                <Tab>
                  <Icon as={FiHome} mr={2} />
                  Vendors ({stats.vendors.length})
                </Tab>
                <Tab>
                  <Icon as={FiUsers} mr={2} />
                  Employees ({stats.employees.length})
                </Tab>
              </TabList>
            </CardHeader>
            
            <CardBody>
              <TabPanels>
                {/* All Contacts Tab */}
                <TabPanel p={0}>
                  {loading ? (
                    <Flex justify="center" py={10}>
                      <VStack>
                        <Spinner size="xl" color="var(--accent-color)" />
                        <Text color={textColor}>Loading all contacts...</Text>
                      </VStack>
                    </Flex>
                  ) : (
                    renderContactList(contacts, 'No contacts found matching your search criteria.')
                  )}
                </TabPanel>

                {/* Customers Tab */}
                <TabPanel p={0}>
                  {loading ? (
                    <Flex justify="center" py={10}>
                      <Spinner size="xl" color="blue.500" />
                    </Flex>
                  ) : (
                    renderContactList(stats.customers, 'No customers found.')
                  )}
                </TabPanel>

                {/* Vendors Tab */}
                <TabPanel p={0}>
                  {loading ? (
                    <Flex justify="center" py={10}>
                      <Spinner size="xl" color="green.500" />
                    </Flex>
                  ) : (
                    renderContactList(stats.vendors, 'No vendors found.')
                  )}
                </TabPanel>

                {/* Employees Tab */}
                <TabPanel p={0}>
                  {loading ? (
                    <Flex justify="center" py={10}>
                      <Spinner size="xl" color="purple.500" />
                    </Flex>
                  ) : (
                    renderContactList(stats.employees, 'No employees found.')
                  )}
                </TabPanel>
              </TabPanels>
            </CardBody>
          </Tabs>
        </Card>

        {/* Action Panel */}
        <Card mt={6}>
          <CardHeader>
            <Heading size="md" color={primaryTextColor}>
              <Icon as={FiBarChart} mr={2} color="var(--accent-color)" />
              Management Tools
            </Heading>
          </CardHeader>
          <CardBody>
            <SimpleGrid columns={{ base: 1, md: 2, lg: 4 }} spacing={4}>
              <Button
                leftIcon={<FiBarChart />}
                colorScheme="blue"
                variant="outline"
                size="lg"
                h={16}
                flexDirection="column"
                _hover={{ bg: 'blue.50', transform: 'translateY(-2px)' }}
              >
                <Text>Generate</Text>
                <Text fontSize="xs">Report</Text>
              </Button>
              
              <Button
                leftIcon={<FiDownload />}
                colorScheme="green"
                variant="outline"
                size="lg"
                h={16}
                flexDirection="column"
                _hover={{ bg: 'green.50', transform: 'translateY(-2px)' }}
              >
                <Text>Export</Text>
                <Text fontSize="xs">Data</Text>
              </Button>
              
              <Button
                leftIcon={<FiUpload />}
                colorScheme="purple"
                variant="outline"
                size="lg"
                h={16}
                flexDirection="column"
                _hover={{ bg: 'purple.50', transform: 'translateY(-2px)' }}
              >
                <Text>Import</Text>
                <Text fontSize="xs">Contacts</Text>
              </Button>
              
              <Button
                leftIcon={<FiSearch />}
                colorScheme="orange"
                variant="outline"
                size="lg"
                h={16}
                flexDirection="column"
                _hover={{ bg: 'orange.50', transform: 'translateY(-2px)' }}
              >
                <Text>Advanced</Text>
                <Text fontSize="xs">Search</Text>
              </Button>
            </SimpleGrid>
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
            <Text>Comprehensive contact form would go here...</Text>
            {/* TODO: Implement comprehensive contact form */}
          </ModalBody>
        </ModalContent>
      </Modal>
    </SimpleLayout>
  );
}
