import React, { useState, useEffect } from 'react';
import {
    Box,
    Button,
    Flex,
    Heading,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    Text,
    useToast,
    IconButton,
    Modal,
    ModalOverlay,
    ModalContent,
    ModalHeader,
    ModalBody,
    ModalCloseButton,
    ModalFooter,
    useDisclosure,
    FormControl,
    FormLabel,
    Input,
    VStack,
    HStack,
    Alert,
    AlertIcon,
    AlertTitle,
    AlertDescription,
    Divider,
    NumberInput,
    NumberInputField,
    NumberInputStepper,
    NumberIncrementStepper,
    NumberDecrementStepper,
} from '@chakra-ui/react';
import { FiPlus, FiTrash2, FiShoppingCart, FiEye } from 'react-icons/fi';
import { projectBOMService, bomService } from '@/services/bomService';
import { ProjectBOM, BOM } from '@/types/bom';
import { formatCurrencyDetailed } from '@/utils/currency';
import SearchableSelect from '@/components/common/SearchableSelect';

interface ProjectBOMListProps {
    projectId: number;
    canEdit: boolean;
    onBudgetUpdated: () => void;
}

const ProjectBOMList: React.FC<ProjectBOMListProps> = ({ projectId, canEdit, onBudgetUpdated }) => {
    const [projectBOMs, setProjectBOMs] = useState<ProjectBOM[]>([]);
    const [availableBOMs, setAvailableBOMs] = useState<BOM[]>([]);
    const [bomOptions, setBomOptions] = useState<{ id: string | number; name: string }[]>([]);
    const [loading, setLoading] = useState(true);
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [isGeneratingPR, setIsGeneratingPR] = useState(false);
    const toast = useToast();

    const { isOpen, onOpen, onClose } = useDisclosure();

    const [selectedBomId, setSelectedBomId] = useState<number>(0);
    const [unitQuantity, setUnitQuantity] = useState<number>(1);

    // Derived: find selected BOM info for display
    const selectedBom = availableBOMs.find(b => b.id === selectedBomId);

    // total_cost di BOM = biaya per 1 unit
    const costPerUnit = selectedBom?.total_cost || 0;
    const totalCostPreview = costPerUnit * unitQuantity;

    useEffect(() => {
        fetchProjectBOMs();
        if (canEdit) {
            fetchAvailableBOMs();
        }
    }, [projectId]);

    const fetchProjectBOMs = async () => {
        try {
            setLoading(true);
            const response = await projectBOMService.getProjectBOMs(projectId);
            if (response.success && response.data) {
                setProjectBOMs(response.data);
            }
        } catch (error: any) {
            toast({
                title: 'Failed to load Project BOMs',
                description: error.message,
                status: 'error',
            });
        } finally {
            setLoading(false);
        }
    };

    const fetchAvailableBOMs = async () => {
        try {
            const response = await bomService.getBOMs();
            if (response.success && response.data) {
                setAvailableBOMs(response.data);
                setBomOptions(
                    response.data.map((bom) => ({
                        id: bom.id,
                        name: `${bom.code} - ${bom.name} → ${bom.target_quantity ?? 1} unit`
                    }))
                );
            }
        } catch (error) {
            console.error('Failed to load Master BOMs', error);
        }
    };

    const handleAddBOM = async () => {
        if (!selectedBomId || unitQuantity < 1) return;
        setIsSubmitting(true);
        try {
            await projectBOMService.addBOMToProject(projectId, { bom_id: selectedBomId, quantity: unitQuantity });
            toast({ title: 'BOM added to project successfully', status: 'success' });
            onClose();
            fetchProjectBOMs();
            onBudgetUpdated();
        } catch (error: any) {
            toast({
                title: 'Failed to add BOM',
                description: error.message,
                status: 'error',
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const handleDeleteBOM = async (projectBomId: number) => {
        if (!window.confirm('Are you sure you want to remove this BOM template from the project? The project budget will be recalculated.')) return;
        try {
            await projectBOMService.removeBOMFromProject(projectId, projectBomId);
            toast({ title: 'BOM removed successfully', status: 'success' });
            fetchProjectBOMs();
            onBudgetUpdated();
        } catch (error: any) {
            toast({
                title: 'Failed to remove BOM',
                description: error.message,
                status: 'error',
            });
        }
    };

    const handleGeneratePR = async () => {
        try {
            setIsGeneratingPR(true);
            await projectBOMService.generatePR(projectId);
            toast({
                title: 'Purchase Request berhasil dibuat',
                description: 'Draft PR telah dibuat dari BOM shortages. Silakan cek menu Purchase Request untuk review dan approval.',
                status: 'success',
                duration: 5000,
            });
        } catch (error: any) {
            toast({
                title: 'Auto-Procurement Check',
                description: error.message,
                status: error.message?.includes('fully stocked') ? 'info' : 'warning',
                duration: 5000,
            });
        } finally {
            setIsGeneratingPR(false);
        }
    };

    const totalAddedBudget = projectBOMs.reduce((sum, pbom) => sum + pbom.total_cost, 0);

    const formatIDR = (value: number) => new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(value);

    return (
        <Box>
            <Flex justify="space-between" align="center" mb={4}>
                <Heading size="sm">BOM Requirements</Heading>
                <HStack spacing={2}>
                    {canEdit && (
                        <Button
                            leftIcon={<FiShoppingCart />}
                            colorScheme="teal"
                            size="sm"
                            variant="outline"
                            onClick={handleGeneratePR}
                            isLoading={isGeneratingPR}
                            loadingText="Checking..."
                        >
                            Generate PR (Stock Check)
                        </Button>
                    )}
                    {canEdit && (
                        <Button
                            leftIcon={<FiPlus />}
                            colorScheme="blue"
                            size="sm"
                            onClick={() => {
                                setSelectedBomId(0);
                                setUnitQuantity(1);
                                onOpen();
                            }}
                        >
                            Add BOM Assignment
                        </Button>
                    )}
                </HStack>
            </Flex>

            <Alert status="info" size="sm" mb={4} borderRadius="md" variant="left-accent">
                <AlertIcon />
                <Box>
                    <AlertDescription fontSize="sm">
                        Adding BOMs automatically aggregates the material costs into the Project's True Budget limit. The <strong>Generate PR</strong> action will match aggregated requests against current Warehouse stock.
                    </AlertDescription>
                </Box>
            </Alert>

            <Box overflowX="auto" borderWidth="1px" borderRadius="md" mb={4}>
                <Table variant="simple" size="sm">
                    <Thead bg="gray.100">
                        <Tr>
                            <Th>Template Name</Th>
                            <Th isNumeric>Qty (Unit)</Th>
                            <Th isNumeric>Cost / Unit</Th>
                            <Th isNumeric>Total Cost</Th>
                            <Th width="80px">Actions</Th>
                        </Tr>
                    </Thead>
                    <Tbody>
                        {loading ? (
                            <Tr><Td colSpan={5} textAlign="center">Loading BOMs...</Td></Tr>
                        ) : projectBOMs.length === 0 ? (
                            <Tr><Td colSpan={5} textAlign="center" color="gray.500">No BOMs attached to this project.</Td></Tr>
                        ) : (
                            projectBOMs.map((pb) => (
                                <Tr key={pb.id}>
                                    <Td fontWeight="medium">{pb.bom_name}</Td>
                                    <Td isNumeric>
                                        <Text fontWeight="medium">{pb.quantity} unit</Text>
                                    </Td>
                                    <Td isNumeric>{formatCurrencyDetailed(pb.unit_cost)}</Td>
                                    <Td isNumeric fontWeight="bold" color="blue.600">{formatCurrencyDetailed(pb.total_cost)}</Td>
                                    <Td>
                                        {canEdit && (
                                            <IconButton
                                                aria-label="Remove assignment"
                                                icon={<FiTrash2 />}
                                                size="sm"
                                                colorScheme="red"
                                                variant="ghost"
                                                onClick={() => handleDeleteBOM(pb.id)}
                                            />
                                        )}
                                    </Td>
                                </Tr>
                            ))
                        )}
                    </Tbody>
                </Table>
            </Box>

            <Flex justify="flex-end" bg="gray.50" p={3} borderRadius="md" borderWidth="1px">
                <Text fontSize="md" fontWeight="bold">
                    Calculated BOM Subtotal: <Text as="span" color="blue.600">{formatCurrencyDetailed(totalAddedBudget)}</Text>
                </Text>
            </Flex>

            <Modal isOpen={isOpen} onClose={onClose}>
                <ModalOverlay />
                <ModalContent>
                    <ModalHeader>Assign BOM Template</ModalHeader>
                    <ModalCloseButton />
                    <ModalBody>
                        <VStack spacing={4}>
                            <FormControl isRequired>
                                <FormLabel>Master BOM Template</FormLabel>
                                <SearchableSelect
                                    options={bomOptions}
                                    value={selectedBomId ? selectedBomId.toString() : ''}
                                    onChange={(val) => {
                                        setSelectedBomId(typeof val === 'string' ? parseInt(val) : (val as number));
                                        setUnitQuantity(1);
                                    }}
                                    placeholder="Select template..."
                                />
                            </FormControl>

                            {selectedBom && (
                                <>
                                    <FormControl isRequired>
                                        <FormLabel>Jumlah Unit</FormLabel>
                                        <NumberInput
                                            min={1}
                                            max={9999}
                                            value={unitQuantity}
                                            onChange={(_, val) => setUnitQuantity(val || 1)}
                                        >
                                            <NumberInputField />
                                            <NumberInputStepper>
                                                <NumberIncrementStepper />
                                                <NumberDecrementStepper />
                                            </NumberInputStepper>
                                        </NumberInput>
                                    </FormControl>

                                    <Box w="100%" p={3} bg="blue.50" borderRadius="md" borderWidth="1px" borderColor="blue.200">
                                        <VStack spacing={1} align="stretch">
                                            <HStack justify="space-between">
                                                <Text fontSize="sm" color="blue.700">Cost / Unit:</Text>
                                                <Text fontSize="sm" color="blue.700">
                                                    {formatIDR(costPerUnit)}
                                                </Text>
                                            </HStack>
                                            <Divider borderColor="blue.200" />
                                            <HStack justify="space-between">
                                                <Text fontSize="sm" fontWeight="bold" color="blue.800">Total ({unitQuantity} unit):</Text>
                                                <Text fontSize="md" fontWeight="bold" color="blue.800">
                                                    {formatIDR(totalCostPreview)}
                                                </Text>
                                            </HStack>
                                        </VStack>
                                    </Box>
                                </>
                            )}
                        </VStack>
                    </ModalBody>
                    <ModalFooter>
                        <Button variant="ghost" mr={3} onClick={onClose} isDisabled={isSubmitting}>Cancel</Button>
                        <Button
                            colorScheme="blue"
                            onClick={handleAddBOM}
                            isLoading={isSubmitting}
                            isDisabled={!selectedBomId || unitQuantity < 1}
                        >
                            Add to Project
                        </Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>

        </Box>
    );
};

export default ProjectBOMList;
