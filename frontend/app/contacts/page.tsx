'use client';

import React, { useEffect, useState } from 'react';
import { useAuth } from '@/contexts/AuthContext';
import { useTranslation } from '@/hooks/useTranslation';
import { useModulePermissions } from '@/hooks/usePermissions';
import ProtectedModule from '@/components/common/ProtectedModule';
import api from '@/services/api';
import { API_ENDPOINTS } from '@/config/api';
import Layout from '@/components/layout/Layout';
import GroupedTable from '@/components/common/GroupedTable';
import {
  Box,
  Flex,
  Heading,
  Button,
  Alert,
  AlertIcon,
  AlertTitle,
  AlertDescription,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalFooter,
  ModalBody,
  ModalCloseButton,
  useToast,
  FormControl,
  FormLabel,
  Input,
  Select,
  Textarea,
  Switch,
  VStack,
  HStack,
  Text,
  useColorModeValue,
} from '@chakra-ui/react';
import { FiPlus, FiEdit, FiTrash2, FiEye } from 'react-icons/fi';

// Define the Contact type
interface Contact {
  id: number;
  code?: string;
  name: string;
  type: 'CUSTOMER' | 'VENDOR' | 'EMPLOYEE';
  category?: string;
  email: string;
  phone: string;
  mobile?: string;
  fax?: string;
  website?: string;
  tax_number?: string;
  credit_limit?: number;
  payment_terms?: number;
  is_active: boolean;
  pic_name?: string;        // Person In Charge (for Customer/Vendor)
  external_id?: string;     // Employee ID, Vendor ID, Customer ID
  address?: string;         // Simple address field
  notes?: string;
  created_at: string;
  updated_at: string;
  addresses?: ContactAddress[];
}

interface ContactAddress {
  id: number;
  contact_id: number;
  type: 'BILLING' | 'SHIPPING' | 'MAILING';
  address1: string;
  address2?: string;
  city: string;
  state?: string;
  postal_code?: string;
  country: string;
  is_default: boolean;
}

const ContactsPage = () => {
  const { token } = useAuth();
  const { t } = useTranslation();

  // Permission-based access control
  const { canEdit } = useModulePermissions('contacts');

  const [contacts, setContacts] = useState<Contact[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [isModalOpen, setIsModalOpen] = useState(false);
  const [selectedContact, setSelectedContact] = useState<Partial<Contact> | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  // Form state
  const [formData, setFormData] = useState<Partial<Contact>>({
    name: '',
    type: 'CUSTOMER',
    email: '',
    phone: '',
    mobile: '',
    notes: '',
    pic_name: '',
    external_id: '',
    address: '',
    is_active: true
  });
  // Fetch contacts from API
  const fetchContacts = async () => {
    try {
      const response = await api.get(API_ENDPOINTS.CONTACTS);
      // Backend returns direct array, not wrapped in data field
      setContacts(Array.isArray(response.data) ? response.data : response.data.data || []);
    } catch (err: any) {
      setError(t('contacts.messages.fetchError'));
      console.error('Error fetching contacts:', err);
    } finally {
      setIsLoading(false);
    }
  };

  // Load contacts on component mount
  useEffect(() => {
    if (token) {
      fetchContacts();
    }
  }, [token]);

  // Handle form submission for create/update
  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setIsSubmitting(true);
    setError(null);

    try {
      if (formData.id) {
        await api.put(`${API_ENDPOINTS.CONTACTS}/${formData.id}`, formData);
      } else {
        await api.post(API_ENDPOINTS.CONTACTS, formData);
      }

      // Refresh contacts list
      fetchContacts();

      // Show success message
      toast({
        title: formData.id ? t('contacts.messages.contactUpdated') : t('contacts.messages.contactCreated'),
        description: formData.id ? t('contacts.messages.updateSuccess') : t('contacts.messages.createSuccess'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });

      // Close modal and reset form
      setIsModalOpen(false);
      setSelectedContact(null);
      setFormData({
        name: '',
        type: 'CUSTOMER',
        email: '',
        phone: '',
        mobile: '',
        notes: '',
        pic_name: '',
        external_id: '',
        address: '',
        is_active: true
      });
    } catch (err) {
      setError(formData.id ? t('contacts.messages.updateError') : t('contacts.messages.createError'));
      console.error('Error submitting contact:', err);
    } finally {
      setIsSubmitting(false);
    }
  };

  // Handle contact deletion
  const handleDelete = async (id: number) => {
    if (!window.confirm(t('contacts.messages.confirmDelete'))) {
      return;
    }

    setIsLoading(true);
    setError(null);

    try {
      await api.delete(`${API_ENDPOINTS.CONTACTS}/${id}`);

      // Refresh contacts list after successful deletion
      fetchContacts();

      // Show success message
      toast({
        title: t('contacts.messages.contactDeleted'),
        description: t('contacts.messages.deleteSuccess'),
        status: 'success',
        duration: 3000,
        isClosable: true,
      });
    } catch (err) {
      setError(t('contacts.messages.deleteError'));
      console.error('Error deleting contact:', err);
    } finally {
      setIsLoading(false);
    }
  };

  // Open modal for creating a new contact
  const handleCreate = () => {
    setSelectedContact(null);
    setFormData({
      name: '',
      type: 'CUSTOMER',
      email: '',
      phone: '',
      mobile: '',
      notes: '',
      pic_name: '',
      external_id: '',
      address: '',
      is_active: true
    });
    setIsModalOpen(true);
  };

  // Open modal for editing an existing contact
  const handleEdit = (contact: Contact) => {
    setSelectedContact(contact);
    setFormData(contact);
    setIsModalOpen(true);
  };

  // Handle form input changes
  const handleInputChange = (field: keyof Contact, value: any) => {
    setFormData(prev => ({
      ...prev,
      [field]: value
    }));
  };

  // Table columns definition (removed Type column since we're grouping by type)
  // Dynamic columns based on contact type
  const getColumnsForType = (contactType?: string) => {
    const baseColumns: Array<{
      header: string;
      accessor: string | ((contact: Contact) => string);
      headerStyle: React.CSSProperties;
      cellStyle: React.CSSProperties;
    }> = [
      {
        header: t('contacts.table.name'),
        accessor: 'name',
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold' },
        cellStyle: { padding: '12px 8px', fontSize: '14px' }
      },
      {
        header: t('contacts.table.externalId'),
        accessor: (contact: Contact) => contact.external_id || '-',
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold', whiteSpace: 'nowrap' },
        cellStyle: { padding: '12px 8px', fontSize: '14px', whiteSpace: 'nowrap' }
      },
    ];

    // Only add PIC Name column for Customer and Vendor groups, not for Employee
    if (contactType !== 'EMPLOYEE') {
      baseColumns.push({
        header: t('contacts.table.picName'),
        accessor: (contact: Contact) => contact.pic_name || '-',
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold', whiteSpace: 'nowrap' },
        cellStyle: { padding: '12px 8px', fontSize: '14px', whiteSpace: 'nowrap' }
      });
    }

    // Add remaining columns
    baseColumns.push(
      {
        header: t('contacts.table.email'),
        accessor: 'email',
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold' },
        cellStyle: { padding: '12px 8px', fontSize: '14px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }
      },
      {
        header: t('contacts.table.phone'),
        accessor: 'phone',
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold', whiteSpace: 'nowrap' },
        cellStyle: { padding: '12px 8px', fontSize: '14px', whiteSpace: 'nowrap' }
      },
      {
        header: t('contacts.table.address'),
        accessor: (contact: Contact) => {
          if (contact.address) {
            // Truncate long address for table display
            return contact.address.length > 50
              ? contact.address.substring(0, 50) + '...'
              : contact.address;
          }
          return '-';
        },
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold' },
        cellStyle: { padding: '12px 8px', fontSize: '14px', maxWidth: '250px', overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }
      },
      {
        header: t('contacts.table.status'),
        accessor: (contact: Contact) => (contact.is_active ? t('contacts.status.active') : t('contacts.status.inactive')),
        headerStyle: { padding: '12px 8px', fontSize: '14px', fontWeight: 'semibold', whiteSpace: 'nowrap' },
        cellStyle: { padding: '12px 8px', fontSize: '14px', whiteSpace: 'nowrap' }
      }
    );

    return baseColumns;
  };

  const toast = useToast();
  const headingColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const modalBg = useColorModeValue('white', 'gray.800');
  const modalHeaderBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const modalBorderColor = useColorModeValue('gray.200', 'whiteAlpha.200');
  const fieldBg = useColorModeValue('white', 'whiteAlpha.50');
  const fieldBorderColor = useColorModeValue('gray.300', 'whiteAlpha.300');
  const fieldTextColor = useColorModeValue('gray.800', 'whiteAlpha.900');
  const labelColor = useColorModeValue('gray.700', 'whiteAlpha.800');
  const placeholderColor = useColorModeValue('gray.400', 'whiteAlpha.500');
  const detailCardBg = useColorModeValue('gray.50', 'whiteAlpha.100');
  const detailMutedText = useColorModeValue('gray.600', 'whiteAlpha.700');
  const selectOptionBg = useColorModeValue('white', 'gray.800');
  const fieldFocusStyles = {
    borderColor: 'blue.400',
    boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)',
  };

  // New state for view modal
  const [isViewModalOpen, setIsViewModalOpen] = useState(false);
  const [viewContact, setViewContact] = useState<Contact | null>(null);

  // Handler to open view modal
  const handleView = (contact: Contact) => {
    setViewContact(contact);
    setIsViewModalOpen(true);
  };

  // Action buttons for each row
  const renderActions = (contact: Contact) => (
    <>
      <Button
        size="xs"
        variant="outline"
        leftIcon={<FiEye />}
        onClick={() => handleView(contact)}
        colorScheme="blue"
        minW="auto"
        px={2}
      >
        {t('common.view') || 'View'}
      </Button>
      {canEdit && (
        <>
          <Button
            size="xs"
            variant="outline"
            leftIcon={<FiEdit />}
            onClick={() => handleEdit(contact)}
            minW="auto"
            px={2}
          >
            {t('common.edit')}
          </Button>
          <Button
            size="xs"
            colorScheme="red"
            variant="outline"
            leftIcon={<FiTrash2 />}
            onClick={() => handleDelete(contact.id)}
            minW="auto"
            px={2}
          >
            {t('common.delete')}
          </Button>
        </>
      )}
    </>
  );

  return (
    <ProtectedModule module="contacts">
      <Layout>
        <Box>
          <Flex justify="space-between" align="center" mb={6}>
            <Heading size="lg" color={headingColor}>
              {t('contacts.contactMaster')}
            </Heading>
            {canEdit && (
              <Button
                colorScheme="brand"
                leftIcon={<FiPlus />}
                onClick={handleCreate}
              >
                {t('contacts.addContact')}
              </Button>
            )}
          </Flex>

          {error && (
            <Alert status="error" mb={4}>
              <AlertIcon />
              <AlertTitle mr={2}>Error!</AlertTitle>
              <AlertDescription>{error}</AlertDescription>
            </Alert>
          )}

          <GroupedTable<Contact>
            columns={getColumnsForType}
            data={contacts}
            keyField="id"
            groupBy="type"
            title={t('contacts.contacts')}
            actions={renderActions}
            isLoading={isLoading}
            groupLabels={{
              VENDOR: t('contacts.vendors'),
              CUSTOMER: t('contacts.customers'),
              EMPLOYEE: t('contacts.employees')
            }}
          />

          <Modal isOpen={isModalOpen} onClose={() => setIsModalOpen(false)} size="lg">
            <ModalOverlay />
            <ModalContent bg={modalBg} borderWidth="1px" borderColor={modalBorderColor} color={fieldTextColor}>
              <form onSubmit={handleSubmit}>
                <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={modalBorderColor}>
                  {selectedContact?.id ? t('contacts.editContact') : t('contacts.createContact')}
                </ModalHeader>
                <ModalCloseButton color={fieldTextColor} />
                <ModalBody>
                  <VStack spacing={4}>
                    <FormControl isRequired>
                      <FormLabel color={labelColor}>{t('common.name')}</FormLabel>
                      <Input
                        value={formData.name || ''}
                        onChange={(e) => handleInputChange('name', e.target.value)}
                        placeholder={t('contacts.form.enterContactName')}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    <FormControl isRequired>
                      <FormLabel color={labelColor}>{t('common.type')}</FormLabel>
                      <Select
                        value={formData.type || 'CUSTOMER'}
                        onChange={(e) => handleInputChange('type', e.target.value as 'CUSTOMER' | 'VENDOR' | 'EMPLOYEE')}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        sx={{ option: { background: selectOptionBg, color: fieldTextColor } }}
                      >
                        <option value="CUSTOMER">{t('contacts.customer')}</option>
                        <option value="VENDOR">{t('contacts.vendor')}</option>
                        <option value="EMPLOYEE">{t('contacts.employee')}</option>
                      </Select>
                    </FormControl>

                    <FormControl>
                      <FormLabel color={labelColor}>
                        {formData.type === 'CUSTOMER' ? t('contacts.customerId') :
                          formData.type === 'VENDOR' ? t('contacts.vendorId') :
                            formData.type === 'EMPLOYEE' ? t('contacts.employeeId') : t('contacts.externalId')}
                      </FormLabel>
                      <Input
                        value={formData.external_id || ''}
                        onChange={(e) => handleInputChange('external_id', e.target.value)}
                        placeholder={t('contacts.form.enterExternalId')}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    {/* PIC Name - only show for Customer/Vendor */}
                    {(formData.type === 'CUSTOMER' || formData.type === 'VENDOR') && (
                      <FormControl>
                        <FormLabel color={labelColor}>{t('contacts.picName')}</FormLabel>
                        <Input
                          value={formData.pic_name || ''}
                          onChange={(e) => handleInputChange('pic_name', e.target.value)}
                          placeholder={t('contacts.form.enterPicName')}
                          bg={fieldBg}
                          borderColor={fieldBorderColor}
                          color={fieldTextColor}
                          _hover={{ borderColor: fieldBorderColor }}
                          _focusVisible={fieldFocusStyles}
                          _placeholder={{ color: placeholderColor }}
                        />
                      </FormControl>
                    )}

                    <FormControl isRequired>
                      <FormLabel color={labelColor}>{t('contacts.email')}</FormLabel>
                      <Input
                        type="email"
                        value={formData.email || ''}
                        onChange={(e) => handleInputChange('email', e.target.value)}
                        placeholder={t('contacts.form.enterEmail')}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    <FormControl isRequired>
                      <FormLabel color={labelColor}>{t('contacts.phone')}</FormLabel>
                      <Input
                        value={formData.phone || ''}
                        onChange={(e) => handleInputChange('phone', e.target.value)}
                        placeholder={t('contacts.form.enterPhone')}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    <FormControl>
                      <FormLabel color={labelColor}>{t('contacts.mobile')}</FormLabel>
                      <Input
                        value={formData.mobile || ''}
                        onChange={(e) => handleInputChange('mobile', e.target.value)}
                        placeholder={t('contacts.form.enterMobile')}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    <FormControl>
                      <FormLabel color={labelColor}>{t('contacts.address')}</FormLabel>
                      <Textarea
                        value={formData.address || ''}
                        onChange={(e) => handleInputChange('address', e.target.value)}
                        placeholder={t('contacts.form.enterAddress')}
                        rows={3}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    <FormControl>
                      <FormLabel color={labelColor}>{t('common.notes')}</FormLabel>
                      <Textarea
                        value={formData.notes || ''}
                        onChange={(e) => handleInputChange('notes', e.target.value)}
                        placeholder={t('contacts.form.enterNotes')}
                        rows={3}
                        bg={fieldBg}
                        borderColor={fieldBorderColor}
                        color={fieldTextColor}
                        _hover={{ borderColor: fieldBorderColor }}
                        _focusVisible={fieldFocusStyles}
                        _placeholder={{ color: placeholderColor }}
                      />
                    </FormControl>

                    <FormControl>
                      <HStack>
                        <FormLabel mb={0} color={labelColor}>{t('common.active')}</FormLabel>
                        <Switch
                          isChecked={formData.is_active !== false}
                          onChange={(e) => handleInputChange('is_active', e.target.checked)}
                        />
                      </HStack>
                    </FormControl>
                  </VStack>
                </ModalBody>
                <ModalFooter borderTopWidth="1px" borderColor={modalBorderColor}>
                  <Button variant="ghost" mr={3} onClick={() => setIsModalOpen(false)}>
                    {t('common.cancel')}
                  </Button>
                  <Button
                    colorScheme="brand"
                    type="submit"
                    isLoading={isSubmitting}
                    loadingText={selectedContact?.id ? t('common.updating') : t('common.creating')}
                  >
                    {selectedContact?.id ? t('common.update') : t('common.create')}
                  </Button>
                </ModalFooter>
              </form>
            </ModalContent>
          </Modal>

          {/* View Modal */}
          <Modal isOpen={isViewModalOpen} onClose={() => setIsViewModalOpen(false)} size="lg">
            <ModalOverlay />
            <ModalContent bg={modalBg} borderWidth="1px" borderColor={modalBorderColor} color={fieldTextColor}>
              <ModalHeader bg={modalHeaderBg} borderBottomWidth="1px" borderColor={modalBorderColor}>
                {t('contacts.contactDetails')}
              </ModalHeader>
              <ModalCloseButton color={fieldTextColor} />
              <ModalBody>
                {viewContact && (
                  <VStack spacing={3} align="stretch">
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('common.name')}</Text>
                      <Text color={fieldTextColor} fontWeight="semibold">{viewContact.name}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('common.type')}</Text>
                      <Text color={fieldTextColor}>{viewContact.type === 'CUSTOMER' ? t('contacts.customer') : viewContact.type === 'VENDOR' ? t('contacts.vendor') : t('contacts.employee')}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.table.code')}</Text>
                      <Text color={fieldTextColor}>{viewContact.code || '-'}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.externalId')}</Text>
                      <Text color={fieldTextColor}>{viewContact.external_id || '-'}</Text>
                    </Box>
                    {(viewContact.type === 'CUSTOMER' || viewContact.type === 'VENDOR') && (
                      <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                        <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.picName')}</Text>
                        <Text color={fieldTextColor}>{viewContact.pic_name || '-'}</Text>
                      </Box>
                    )}
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.email')}</Text>
                      <Text color={fieldTextColor}>{viewContact.email}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.phone')}</Text>
                      <Text color={fieldTextColor}>{viewContact.phone}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.mobile')}</Text>
                      <Text color={fieldTextColor}>{viewContact.mobile || '-'}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.address')}</Text>
                      <Text color={fieldTextColor}>{viewContact.address || '-'}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('common.status')}</Text>
                      <Text color={fieldTextColor}>{viewContact.is_active ? t('contacts.status.active') : t('contacts.status.inactive')}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('common.notes')}</Text>
                      <Text color={fieldTextColor}>{viewContact.notes || '-'}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.table.created')}</Text>
                      <Text color={fieldTextColor}>{new Date(viewContact.created_at).toLocaleDateString()}</Text>
                    </Box>
                    <Box bg={detailCardBg} borderWidth="1px" borderColor={modalBorderColor} borderRadius="md" p={4}>
                      <Text color={detailMutedText} fontSize="sm" mb={1}>{t('contacts.table.updated')}</Text>
                      <Text color={fieldTextColor}>{new Date(viewContact.updated_at).toLocaleDateString()}</Text>
                    </Box>
                  </VStack>
                )}
              </ModalBody>
              <ModalFooter borderTopWidth="1px" borderColor={modalBorderColor}>
                <Button onClick={() => setIsViewModalOpen(false)}>{t('common.close')}</Button>
              </ModalFooter>
            </ModalContent>
          </Modal>
        </Box>
      </Layout>
    </ProtectedModule>
  );
};

export default ContactsPage;
