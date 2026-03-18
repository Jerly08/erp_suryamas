import React, { useState, useEffect } from 'react';
import {
  Box,
  Button,
  Table,
  Thead,
  Tbody,
  Tr,
  Th,
  Td,
  useToast,
  HStack,
  Badge,
  Input,
  InputGroup,
  InputLeftElement,
  Flex,
  Modal,
  ModalOverlay,
  ModalContent,
  ModalHeader,
  ModalBody,
  ModalCloseButton,
  useDisclosure,
  AlertDialog,
  AlertDialogBody,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogContent,
  AlertDialogOverlay,
  Text,
} from '@chakra-ui/react';
import { FiEdit, FiTrash2, FiPlus, FiSearch } from 'react-icons/fi';
import ProductService, { ProductUnit } from '@/services/productService';
import UnitForm from './UnitForm';
import { useTranslation } from '@/hooks/useTranslation';

const UnitManagement: React.FC = () => {
  const { t } = useTranslation();
  const [units, setUnits] = useState<ProductUnit[]>([]);
  const [filteredUnits, setFilteredUnits] = useState<ProductUnit[]>([]);
  const [searchTerm, setSearchTerm] = useState('');
  const [selectedUnit, setSelectedUnit] = useState<ProductUnit | null>(null);
  const [unitToDelete, setUnitToDelete] = useState<ProductUnit | null>(null);
  const { isOpen: isFormOpen, onOpen: onFormOpen, onClose: onFormClose } = useDisclosure();
  const { isOpen: isDeleteOpen, onOpen: onDeleteOpen, onClose: onDeleteClose } = useDisclosure();
  const cancelRef = React.useRef<HTMLButtonElement>(null);
  const toast = useToast();

  useEffect(() => {
    fetchUnits();
  }, []);

  useEffect(() => {
    if (searchTerm) {
      const filtered = units.filter(unit =>
        unit.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        unit.code.toLowerCase().includes(searchTerm.toLowerCase()) ||
        (unit.symbol && unit.symbol.toLowerCase().includes(searchTerm.toLowerCase()))
      );
      setFilteredUnits(filtered);
    } else {
      setFilteredUnits(units);
    }
  }, [searchTerm, units]);

  const fetchUnits = async () => {
    try {
      const data = await ProductService.getProductUnits();
      setUnits(data.data || []);
    } catch (error) {
      toast({
        title: t('products.management.fetchFailed') + ' ' + t('products.management.units').toLowerCase(),
        status: 'error',
        isClosable: true,
      });
    }
  };

  const handleAddClick = () => {
    setSelectedUnit(null);
    onFormOpen();
  };

  const handleEditClick = (unit: ProductUnit) => {
    setSelectedUnit(unit);
    onFormOpen();
  };

  const handleDeleteClick = (unit: ProductUnit) => {
    setUnitToDelete(unit);
    onDeleteOpen();
  };

  const confirmDelete = async () => {
    if (!unitToDelete?.id) return;

    try {
      await ProductService.deleteProductUnit(unitToDelete.id);
      toast({
        title: t('products.management.unitDeleted'),
        status: 'success',
        isClosable: true,
      });
      fetchUnits();
      onDeleteClose();
    } catch (error: any) {
      toast({
        title: t('products.management.deleteFailed') + ' ' + t('products.management.units').toLowerCase(),
        description: error?.response?.data?.error || 'An error occurred',
        status: 'error',
        isClosable: true,
      });
    }
  };

  const handleSaveUnit = (unit: ProductUnit) => {
    fetchUnits();
    onFormClose();
    setSelectedUnit(null);
  };

  const handleCancelForm = () => {
    onFormClose();
    setSelectedUnit(null);
  };

  return (
    <Box>
      {/* Header with Add Button and Search */}
      <Flex justify="space-between" align="center" mb={4}>
        <Button
          leftIcon={<FiPlus />}
          colorScheme="purple"
          onClick={handleAddClick}
        >
          {t('products.management.addUnit')}
        </Button>

        <InputGroup maxW="300px">
          <InputLeftElement pointerEvents="none">
            <FiSearch color="gray.300" />
          </InputLeftElement>
          <Input
            placeholder={t('products.management.searchUnits')}
            value={searchTerm}
            onChange={(e) => setSearchTerm(e.target.value)}
          />
        </InputGroup>
      </Flex>

      {/* Units Table */}
      <Box overflowX="auto">
        <Table variant="simple" size="sm">
          <Thead>
            <Tr>
              <Th>{t('products.table.code')}</Th>
              <Th>{t('products.table.name')}</Th>
              <Th>{t('products.table.symbol')}</Th>
              <Th>{t('products.table.type')}</Th>
              <Th>{t('products.table.description')}</Th>
              <Th>{t('products.table.status')}</Th>
              <Th>{t('products.table.actions')}</Th>
            </Tr>
          </Thead>
          <Tbody>
            {filteredUnits.length === 0 ? (
              <Tr>
                <Td colSpan={7} textAlign="center" py={8}>
                  <Text color="gray.500">
                    {searchTerm ? t('products.management.noUnitsFound') : t('products.management.noUnitsYet')}
                  </Text>
                </Td>
              </Tr>
            ) : (
              filteredUnits.map((unit) => (
                <Tr key={unit.id}>
                  <Td fontWeight="medium">{unit.code}</Td>
                  <Td>{unit.name}</Td>
                  <Td>{unit.symbol || '-'}</Td>
                  <Td>{unit.type || '-'}</Td>
                  <Td>{unit.description || '-'}</Td>
                  <Td>
                    <Badge colorScheme={unit.is_active ? 'green' : 'red'}>
                      {unit.is_active ? t('common.active') : t('common.inactive')}
                    </Badge>
                  </Td>
                  <Td>
                    <HStack spacing={2}>
                      <Button
                        size="sm"
                        leftIcon={<FiEdit />}
                        colorScheme="blue"
                        variant="ghost"
                        onClick={() => handleEditClick(unit)}
                      >
                        {t('common.edit')}
                      </Button>
                      <Button
                        size="sm"
                        leftIcon={<FiTrash2 />}
                        colorScheme="red"
                        variant="ghost"
                        onClick={() => handleDeleteClick(unit)}
                      >
                        {t('common.delete')}
                      </Button>
                    </HStack>
                  </Td>
                </Tr>
              ))
            )}
          </Tbody>
        </Table>
      </Box>

      {/* Add/Edit Unit Modal */}
      <Modal isOpen={isFormOpen} onClose={onFormClose} size="3xl">
        <ModalOverlay />
        <ModalContent>
          <ModalHeader>
            {selectedUnit ? t('common.edit') + ' ' + t('products.unit') : t('common.add') + ' ' + t('products.unit')}
          </ModalHeader>
          <ModalCloseButton />
          <ModalBody pb={6}>
            <UnitForm
              unit={selectedUnit || undefined}
              onSave={handleSaveUnit}
              onCancel={handleCancelForm}
            />
          </ModalBody>
        </ModalContent>
      </Modal>

      {/* Delete Confirmation Dialog */}
      <AlertDialog
        isOpen={isDeleteOpen}
        leastDestructiveRef={cancelRef}
        onClose={onDeleteClose}
      >
        <AlertDialogOverlay>
          <AlertDialogContent>
            <AlertDialogHeader fontSize="lg" fontWeight="bold">
              {t('products.management.deleteUnit')}
            </AlertDialogHeader>

            <AlertDialogBody>
              {t('products.management.confirmDeleteUnit')} <strong>{unitToDelete?.name}</strong>? 
              {t('products.management.cannotBeUndone')}
            </AlertDialogBody>

            <AlertDialogFooter>
              <Button ref={cancelRef} onClick={onDeleteClose}>
                {t('common.cancel')}
              </Button>
              <Button colorScheme="red" onClick={confirmDelete} ml={3}>
                {t('common.delete')}
              </Button>
            </AlertDialogFooter>
          </AlertDialogContent>
        </AlertDialogOverlay>
      </AlertDialog>
    </Box>
  );
};

export default UnitManagement;
