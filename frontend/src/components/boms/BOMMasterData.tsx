import React, { useState, useEffect } from 'react';
import {
    Box,
    Button,
    Heading,
    Table,
    Tbody,
    Td,
    Th,
    Thead,
    Tr,
    Flex,
    useToast,
    HStack,
    Text,
    Badge,
    Input,
    InputGroup,
    InputLeftElement,
    Icon,
} from '@chakra-ui/react';
import { FiPlus, FiSearch, FiEdit, FiTrash2, FiEye } from 'react-icons/fi';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useModulePermissions } from '@/hooks/usePermissions';
import { formatCurrencyDetailed } from '@/utils/currency';
import { bomService } from '@/services/bomService';
import { BOM } from '@/types/bom';
import BOMFormModal from './BOMFormModal';

const BOMMasterData: React.FC = () => {
    const { canCreate, canEdit, canDelete } = useModulePermissions('boms');
    const [boms, setBoms] = useState<BOM[]>([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [selectedBOM, setSelectedBOM] = useState<BOM | null>(null);
    const [isViewOnly, setIsViewOnly] = useState(false);
    const toast = useToast();

    useEffect(() => {
        fetchBOMs();
    }, []);

    const fetchBOMs = async () => {
        try {
            const response = await bomService.getBOMs();
            if (response.success && response.data) {
                setBoms(response.data);
            }
        } catch (error: any) {
            toast({
                title: 'Error loading BOM templates',
                description: error.message,
                status: 'error',
                isClosable: true,
            });
        }
    };

    const handleSearch = (e: React.ChangeEvent<HTMLInputElement>) => {
        setSearchTerm(e.target.value);
    };

    const filteredBOMs = boms.filter((bom) =>
        bom.name.toLowerCase().includes(searchTerm.toLowerCase()) ||
        bom.code.toLowerCase().includes(searchTerm.toLowerCase())
    );

    const handleAddClick = () => {
        if (!canCreate) return;
        setSelectedBOM(null);
        setIsViewOnly(false);
        setIsModalOpen(true);
    };

    const handleEditClick = (bom: BOM) => {
        if (!canEdit) return;
        setSelectedBOM(bom);
        setIsViewOnly(false);
        setIsModalOpen(true);
    };

    const handleViewClick = (bom: BOM) => {
        setSelectedBOM(bom);
        setIsViewOnly(true);
        setIsModalOpen(true);
    };

    const handleDelete = async (id: number) => {
        if (!canDelete || !window.confirm('Are you sure you want to delete this BOM template?')) return;
        try {
            await bomService.deleteBOM(id);
            toast({
                title: 'BOM deleted successfully',
                status: 'success',
                isClosable: true,
            });
            fetchBOMs();
        } catch (error: any) {
            toast({
                title: 'Failed to delete BOM',
                description: error.message,
                status: 'error',
                isClosable: true,
            });
        }
    };

    return (
        <SimpleLayout title="Bill of Materials (BOM) Master Data">
            <Box bg="white" p={6} borderRadius="lg" shadow="sm">
                <Flex justify="space-between" align="center" mb={6}>
                    <Heading size="md" color="gray.700">Master BOM Templates</Heading>
                    {canCreate && (
                        <Button
                            leftIcon={<FiPlus />}
                            colorScheme="blue"
                            onClick={handleAddClick}
                        >
                            Add New BOM
                        </Button>
                    )}
                </Flex>

                <Flex mb={4} gap={4}>
                    <InputGroup maxW="300px">
                        <InputLeftElement pointerEvents="none">
                            <Icon as={FiSearch} color="gray.400" />
                        </InputLeftElement>
                        <Input
                            type="text"
                            placeholder="Search BOM code or name..."
                            value={searchTerm}
                            onChange={handleSearch}
                        />
                    </InputGroup>
                </Flex>

                <Box overflowX="auto">
                    <Table variant="simple" size="sm">
                        <Thead>
                            <Tr bg="gray.50">
                                <Th>Code</Th>
                                <Th>Name</Th>
                                <Th>Target Product</Th>
                                <Th isNumeric>Target Qty</Th>
                                <Th isNumeric>Cost / Unit</Th>
                                <Th isNumeric>Batch Cost</Th>
                                <Th>Items Count</Th>
                                <Th textAlign="center">Actions</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {filteredBOMs.length === 0 ? (
                                <Tr>
                                    <Td colSpan={8} textAlign="center" py={4} color="gray.500">
                                        No BOM templates found.
                                    </Td>
                                </Tr>
                            ) : (
                                filteredBOMs.map((bom) => (
                                    <Tr key={bom.id} _hover={{ bg: 'gray.50' }}>
                                        <Td fontWeight="medium">{bom.code}</Td>
                                        <Td>{bom.name}</Td>
                                        <Td>{bom.target_product?.name || `Product ID: ${bom.target_product_id}`}</Td>
                                        <Td isNumeric>{bom.target_quantity}</Td>
                                        <Td isNumeric fontWeight="bold" color="blue.600">
                                            {formatCurrencyDetailed(bom.total_cost)}
                                        </Td>
                                        <Td isNumeric fontWeight="bold" color={(bom.target_quantity || 1) > 1 ? 'orange.500' : 'blue.600'}>
                                            {formatCurrencyDetailed(bom.total_cost * (bom.target_quantity || 1))}
                                        </Td>
                                        <Td>
                                            <Badge colorScheme="purple" variant="subtle">
                                                {bom.items?.length || 0} Components
                                            </Badge>
                                        </Td>
                                        <Td>
                                            <HStack spacing={2} justify="center">
                                                <Button
                                                    size="sm"
                                                    variant="ghost"
                                                    colorScheme="gray"
                                                    onClick={() => handleViewClick(bom)}
                                                    title="View Details"
                                                >
                                                    <Icon as={FiEye} />
                                                </Button>
                                                {canEdit && (
                                                    <Button
                                                        size="sm"
                                                        variant="ghost"
                                                        colorScheme="blue"
                                                        onClick={() => handleEditClick(bom)}
                                                        title="Edit"
                                                    >
                                                        <Icon as={FiEdit} />
                                                    </Button>
                                                )}
                                                {canDelete && (
                                                    <Button
                                                        size="sm"
                                                        variant="ghost"
                                                        colorScheme="red"
                                                        onClick={() => handleDelete(bom.id)}
                                                        title="Delete"
                                                    >
                                                        <Icon as={FiTrash2} />
                                                    </Button>
                                                )}
                                            </HStack>
                                        </Td>
                                    </Tr>
                                ))
                            )}
                        </Tbody>
                    </Table>
                </Box>
            </Box>

            {isModalOpen && (
                <BOMFormModal
                    isOpen={isModalOpen}
                    onClose={() => setIsModalOpen(false)}
                    onSuccess={fetchBOMs}
                    existingBOM={selectedBOM}
                    isViewOnly={isViewOnly}
                />
            )}
        </SimpleLayout>
    );
};

export default BOMMasterData;
