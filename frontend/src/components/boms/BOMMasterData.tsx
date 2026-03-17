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
    Badge,
    Input,
    InputGroup,
    InputLeftElement,
    Icon,
    useColorModeValue,
} from '@chakra-ui/react';
import { FiPlus, FiSearch, FiEdit, FiTrash2, FiEye, FiClipboard } from 'react-icons/fi';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useModulePermissions } from '@/hooks/usePermissions';
import { formatCurrencyDetailed } from '@/utils/currency';
import { bomService } from '@/services/bomService';
import { BOM } from '@/types/bom';
import BOMFormModal from './BOMFormModal';
import ComponentTemplateManagerModal from './ComponentTemplateManagerModal';

const BOMMasterData: React.FC = () => {
    const { canCreate, canEdit, canDelete } = useModulePermissions('boms');
    const [boms, setBoms] = useState<BOM[]>([]);
    const [searchTerm, setSearchTerm] = useState('');
    const [isModalOpen, setIsModalOpen] = useState(false);
    const [selectedBOM, setSelectedBOM] = useState<BOM | null>(null);
    const [isViewOnly, setIsViewOnly] = useState(false);
    const [templateManagerOpen, setTemplateManagerOpen] = useState(false);
    const toast = useToast();
    const cardBg = useColorModeValue('white', 'var(--bg-secondary)');
    const headingColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const textColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const mutedTextColor = useColorModeValue('gray.500', 'var(--text-secondary)');
    const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
    const tableHeadBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');
    const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.50');
    const inputBg = useColorModeValue('white', 'var(--bg-secondary)');
    const searchIconColor = useColorModeValue('gray.400', 'gray.500');
    const hoverBorderColor = useColorModeValue('gray.300', 'gray.500');
    const costColor = useColorModeValue('blue.600', 'blue.300');
    const batchCostColor = useColorModeValue('orange.500', 'orange.300');

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
            <Box bg={cardBg} p={6} borderRadius="lg" shadow="sm" border="1px solid" borderColor={borderColor}>
                <Flex justify="space-between" align="center" mb={6}>
                    <Heading size="md" color={headingColor}>Master BOM Templates</Heading>
                    <HStack spacing={2}>
                        <Button
                            leftIcon={<FiClipboard />}
                            colorScheme="green"
                            variant="outline"
                            onClick={() => setTemplateManagerOpen(true)}
                        >
                            Manage Templates
                        </Button>
                        {canCreate && (
                            <Button
                                leftIcon={<FiPlus />}
                                colorScheme="blue"
                                onClick={handleAddClick}
                            >
                                Add New BOM
                            </Button>
                        )}
                    </HStack>
                </Flex>

                <Flex mb={4} gap={4}>
                    <InputGroup maxW="300px">
                        <InputLeftElement pointerEvents="none">
                            <Icon as={FiSearch} color={searchIconColor} />
                        </InputLeftElement>
                        <Input
                            type="text"
                            placeholder="Search BOM code or name..."
                            value={searchTerm}
                            onChange={handleSearch}
                            bg={inputBg}
                            color={textColor}
                            borderColor={borderColor}
                            _hover={{ borderColor: hoverBorderColor }}
                            _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
                            _placeholder={{ color: mutedTextColor }}
                        />
                    </InputGroup>
                </Flex>

                <Box overflowX="auto" border="1px solid" borderColor={borderColor} borderRadius="md">
                    <Table variant="simple" size="sm">
                        <Thead bg={tableHeadBg}>
                            <Tr>
                                <Th color={mutedTextColor} borderColor={borderColor}>Code</Th>
                                <Th color={mutedTextColor} borderColor={borderColor}>Name</Th>
                                <Th color={mutedTextColor} borderColor={borderColor}>Target Product</Th>
                                <Th isNumeric color={mutedTextColor} borderColor={borderColor}>Target Qty</Th>
                                <Th isNumeric color={mutedTextColor} borderColor={borderColor}>Cost / Unit</Th>
                                <Th isNumeric color={mutedTextColor} borderColor={borderColor}>Batch Cost</Th>
                                <Th color={mutedTextColor} borderColor={borderColor}>Items Count</Th>
                                <Th textAlign="center" color={mutedTextColor} borderColor={borderColor}>Actions</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {filteredBOMs.length === 0 ? (
                                <Tr>
                                    <Td colSpan={8} textAlign="center" py={4} color={mutedTextColor} borderColor={borderColor}>
                                        No BOM templates found.
                                    </Td>
                                </Tr>
                            ) : (
                                filteredBOMs.map((bom) => (
                                    <Tr key={bom.id} _hover={{ bg: rowHoverBg }}>
                                        <Td fontWeight="medium" color={textColor} borderColor={borderColor}>{bom.code}</Td>
                                        <Td color={textColor} borderColor={borderColor}>{bom.name}</Td>
                                        <Td color={textColor} borderColor={borderColor}>{bom.target_product?.name || `Product ID: ${bom.target_product_id}`}</Td>
                                        <Td isNumeric color={textColor} borderColor={borderColor}>{bom.target_quantity}</Td>
                                        <Td isNumeric fontWeight="bold" color={costColor} borderColor={borderColor}>
                                            {formatCurrencyDetailed(bom.total_cost)}
                                        </Td>
                                        <Td isNumeric fontWeight="bold" color={(bom.target_quantity || 1) > 1 ? batchCostColor : costColor} borderColor={borderColor}>
                                            {formatCurrencyDetailed(bom.total_cost * (bom.target_quantity || 1))}
                                        </Td>
                                        <Td borderColor={borderColor}>
                                            <Badge colorScheme="purple" variant="subtle">
                                                {bom.items?.length || 0} Components
                                            </Badge>
                                        </Td>
                                        <Td borderColor={borderColor}>
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

            <ComponentTemplateManagerModal
                isOpen={templateManagerOpen}
                onClose={() => setTemplateManagerOpen(false)}
            />
        </SimpleLayout>
    );
};

export default BOMMasterData;
