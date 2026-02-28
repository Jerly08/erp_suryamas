import React, { useState, useEffect } from 'react';
import {
    Modal,
    ModalOverlay,
    ModalContent,
    ModalHeader,
    ModalCloseButton,
    ModalBody,
    ModalFooter,
    Button,
    FormControl,
    FormLabel,
    Input,
    VStack,
    HStack,
    Flex,
    FormErrorMessage,
    useToast,
    Textarea,
    Box,
    Heading,
    Table,
    Thead,
    Tbody,
    Tfoot,
    Tr,
    Th,
    Td,
    IconButton,
    NumberInput,
    NumberInputField,
    Text,
    Divider,
    Badge,
    SimpleGrid,
} from '@chakra-ui/react';
import { FiPlus, FiTrash2 } from 'react-icons/fi';
import { useForm, Controller, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';

import { bomService } from '@/services/bomService';
import ProductService, { Product } from '@/services/productService';
import { BOM } from '@/types/bom';
import SearchableSelect from '@/components/common/SearchableSelect';
import CurrencyInput from '@/components/common/CurrencyInput';

const bomItemSchema = z.object({
    component_product_id: z.number().min(1, 'Product is required'),
    category_str: z.string().min(1, 'Category is required'),
    base_quantity: z.number().min(0, 'Quantity must be >= 0'),
    base_unit: z.string().min(1, 'Unit is required'),
    notes: z.string().optional(),
    unit_price: z.number().min(0, 'Price must be >= 0'),
    requirement_quantity: z.number().min(0.01, 'Req qty must be > 0'),
});

const formSchema = z.object({
    code: z.string().min(1, 'Code is required'),
    name: z.string().min(1, 'Name is required'),
    description: z.string().optional(),
    target_product_id: z.number().min(1, 'Target product is required'),
    target_quantity: z.number().min(1, 'Quantity must be at least 1'),
    items: z.array(bomItemSchema).min(1, 'At least one component is required'),
});

type FormData = z.infer<typeof formSchema>;

interface BOMFormModalProps {
    isOpen: boolean;
    onClose: () => void;
    onSuccess: () => void;
    existingBOM?: BOM | null;
    isViewOnly?: boolean;
}

// --- View-Only Modal Content ---
const BOMViewContent: React.FC<{ bom: BOM }> = ({ bom }) => {
    const formatCurrency = (val: number) =>
        new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(val);

    return (
        <VStack spacing={5} align="stretch">
            {/* Header Box */}
            <Box p={4} borderWidth="1px" borderRadius="md" bg="gray.50">
                <Heading size="sm" mb={4} color="gray.700">Header Information</Heading>
                <SimpleGrid columns={2} spacing={4} mb={4}>
                    <Box>
                        <Text fontSize="xs" color="gray.500" fontWeight="semibold" mb={1}>BOM CODE</Text>
                        <Text fontWeight="bold" fontSize="md">{bom.code}</Text>
                    </Box>
                    <Box>
                        <Text fontSize="xs" color="gray.500" fontWeight="semibold" mb={1}>BOM NAME</Text>
                        <Text fontWeight="bold" fontSize="md">{bom.name}</Text>
                    </Box>
                    <Box>
                        <Text fontSize="xs" color="gray.500" fontWeight="semibold" mb={1}>TARGET ASSEMBLY PRODUCT</Text>
                        <Text fontSize="md">{bom.target_product?.name || `Product ID: ${bom.target_product_id}`}</Text>
                    </Box>
                    <Box>
                        <Text fontSize="xs" color="gray.500" fontWeight="semibold" mb={1}>TARGET QUANTITY</Text>
                        <Text fontSize="md">{bom.target_quantity}</Text>
                    </Box>
                </SimpleGrid>
                {bom.description && (
                    <Box>
                        <Text fontSize="xs" color="gray.500" fontWeight="semibold" mb={1}>DESCRIPTION / NOTES</Text>
                        <Text fontSize="sm" color="gray.700" whiteSpace="pre-wrap">{bom.description}</Text>
                    </Box>
                )}
            </Box>

            <Divider />

            {/* Components Table */}
            <Box>
                <Flex justify="space-between" align="center" mb={3}>
                    <Heading size="sm" color="gray.700">Components (Base Cost Calculator)</Heading>
                    <Badge colorScheme="purple" variant="subtle" fontSize="sm" px={3} py={1} borderRadius="full">
                        {bom.items?.length || 0} Components
                    </Badge>
                </Flex>

                <Box overflowX="auto" borderWidth="1px" borderRadius="md">
                    <Table size="sm" variant="simple">
                        <Thead bg="gray.100">
                            <Tr>
                                <Th minW="160px">Material</Th>
                                <Th minW="110px">Category</Th>
                                <Th minW="110px">Qty / Unit</Th>
                                <Th minW="100px">Notes</Th>
                                <Th minW="70px" isNumeric>Req Qty</Th>
                                <Th minW="120px" isNumeric>Unit Price</Th>
                                <Th minW="120px" isNumeric>Total Cost</Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {bom.items && bom.items.length > 0 ? (
                                bom.items.map((item, idx) => {
                                    const lineTotal = (item.requirement_quantity || 0) * (item.unit_price || 0);
                                    return (
                                        <Tr key={item.id ?? idx} _hover={{ bg: 'gray.50' }}>
                                            <Td>
                                                <Text fontWeight="medium" fontSize="sm">
                                                    {item.component_product?.name || item.component_product?.code
                                                        ? `${item.component_product.code ? item.component_product.code + ' - ' : ''}${item.component_product.name}`
                                                        : `Product ID: ${item.component_product_id}`}
                                                </Text>
                                            </Td>
                                            <Td>
                                                <Badge colorScheme="blue" variant="subtle" fontSize="xs">
                                                    {item.category_str || '—'}
                                                </Badge>
                                            </Td>
                                            <Td>
                                                <Text fontSize="sm" color="gray.700">
                                                    {item.base_quantity} {item.base_unit || ''}
                                                </Text>
                                            </Td>
                                            <Td>
                                                <Text fontSize="xs" color="gray.600">{item.notes || '—'}</Text>
                                            </Td>
                                            <Td isNumeric>{item.requirement_quantity}</Td>
                                            <Td isNumeric>{formatCurrency(item.unit_price || 0)}</Td>
                                            <Td isNumeric fontWeight="bold" color="blue.600">
                                                {formatCurrency(lineTotal)}
                                            </Td>
                                        </Tr>
                                    );
                                })
                            ) : (
                                <Tr>
                                    <Td colSpan={8} textAlign="center" py={4} color="gray.400">
                                        No components found.
                                    </Td>
                                </Tr>
                            )}
                        </Tbody>
                        <Tfoot>
                            <Tr bg="blue.50">
                                <Td colSpan={6} textAlign="right" fontWeight="bold" fontSize="sm" color="gray.700" borderTop="1px" borderColor="blue.200">
                                    Total Unit Cost:
                                </Td>
                                <Td isNumeric fontWeight="bold" fontSize="md" color="blue.700" borderTop="1px" borderColor="blue.200">
                                    {formatCurrency(bom.total_cost || 0)}
                                </Td>
                            </Tr>
                        </Tfoot>
                    </Table>
                </Box>
            </Box>
        </VStack>
    );
};

// --- Main Modal ---
const BOMFormModal: React.FC<BOMFormModalProps> = ({
    isOpen,
    onClose,
    onSuccess,
    existingBOM,
    isViewOnly = false,
}) => {
    const [isSubmitting, setIsSubmitting] = useState(false);
    const [products, setProducts] = useState<Product[]>([]);
    const [productOptions, setProductOptions] = useState<{ id: string | number; name: string }[]>([]);
    const toast = useToast();

    const {
        control,
        handleSubmit,
        register,
        reset,
        watch,
        setValue,
        formState: { errors },
    } = useForm<FormData>({
        resolver: zodResolver(formSchema),
        defaultValues: {
            code: '',
            name: '',
            description: '',
            target_product_id: 0,
            target_quantity: 1,
            items: [],
        },
    });

    const { fields, append, remove } = useFieldArray({
        control,
        name: 'items',
    });

    const watchedItems = watch('items');

    useEffect(() => {
        if (!isViewOnly) {
            fetchProducts();
        }
    }, [isViewOnly]);

    useEffect(() => {
        if (existingBOM && isOpen && !isViewOnly) {
            reset({
                code: existingBOM.code,
                name: existingBOM.name,
                description: existingBOM.description,
                target_product_id: existingBOM.target_product_id,
                target_quantity: existingBOM.target_quantity,
                items: existingBOM.items.map((item) => ({
                    component_product_id: item.component_product_id,
                    category_str: item.category_str,
                    base_quantity: item.base_quantity,
                    base_unit: item.base_unit,
                    notes: item.notes,
                    unit_price: item.unit_price,
                    requirement_quantity: item.requirement_quantity,
                })),
            });
        } else if (isOpen && !isViewOnly) {
            reset({
                code: '',
                name: '',
                description: '',
                target_product_id: 0,
                target_quantity: 1,
                items: [],
            });
        }
    }, [existingBOM, isOpen, isViewOnly, reset]);

    const fetchProducts = async () => {
        try {
            const response = await ProductService.getProducts({ limit: 1000 });
            if (response.data) {
                setProducts(response.data);
                setProductOptions(
                    response.data.map((p: Product) => ({ id: p.id, name: `${p.code} - ${p.name}` }))
                );
            }
        } catch (error) {
            console.error('Failed to fetch products', error);
        }
    };

    const handleProductSelect = (index: number, productIdStr: string | number) => {
        const pId = typeof productIdStr === 'string' ? parseInt(productIdStr) : productIdStr;
        const product = products.find((p) => p.id === pId);
        if (product) {
            setValue(`items.${index}.component_product_id`, pId);
            setValue(`items.${index}.unit_price`, product.purchase_price || 0);
            setValue(`items.${index}.base_unit`, product.unit || '');
            // Auto-fill current stock — user can see if order is needed
            setValue(`items.${index}.base_quantity`, product.stock ?? 0);
        }
    };

    const onSubmit = async (data: FormData) => {
        setIsSubmitting(true);
        const payload = {
            ...data,
            description: data.description || '',
            items: data.items.map(item => ({
                ...item,
                notes: item.notes || ''
            }))
        };
        try {
            if (existingBOM) {
                await bomService.updateBOM(existingBOM.id, payload);
                toast({
                    title: 'BOM updated successfully',
                    status: 'success',
                    isClosable: true,
                });
            } else {
                await bomService.createBOM(payload);
                toast({
                    title: 'BOM created successfully',
                    status: 'success',
                    isClosable: true,
                });
            }
            onSuccess();
            onClose();
        } catch (error: any) {
            toast({
                title: 'Failed to save BOM',
                description: error.message,
                status: 'error',
                isClosable: true,
            });
        } finally {
            setIsSubmitting(false);
        }
    };

    const watchedTargetQty = watch('target_quantity');

    // Cost per 1 unit = sum(reqQty * unitPrice) for all components
    const calculateTotal = () => {
        return watchedItems.reduce((sum, item) => {
            return sum + (item.requirement_quantity || 0) * (item.unit_price || 0);
        }, 0);
    };

    // Total cost for full production batch = unitCost * TargetQty
    const calculateBatchTotal = () => {
        return calculateTotal() * (watchedTargetQty || 1);
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} size="6xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent my={4} maxH="90vh">
                {/* ===== VIEW ONLY MODE ===== */}
                {isViewOnly && existingBOM ? (
                    <>
                        <ModalHeader borderBottomWidth="1px">
                            <HStack spacing={3}>
                                <Text>View BOM Template</Text>
                                <Badge colorScheme="gray" variant="outline">{existingBOM.code}</Badge>
                            </HStack>
                        </ModalHeader>
                        <ModalCloseButton />
                        <ModalBody pb={6}>
                            <BOMViewContent bom={existingBOM} />
                        </ModalBody>
                        <ModalFooter borderTopWidth="1px" bg="gray.50">
                            <Button colorScheme="blue" variant="ghost" onClick={onClose}>
                                Close
                            </Button>
                        </ModalFooter>
                    </>
                ) : (
                    /* ===== CREATE / EDIT MODE ===== */
                    <form
                        onSubmit={handleSubmit(onSubmit)}
                        style={{ display: 'flex', flexDirection: 'column', flex: 1, minHeight: 0, overflow: 'hidden' }}
                    >
                        <ModalHeader>
                            {existingBOM ? 'Edit BOM Template' : 'Create New BOM Template'}
                        </ModalHeader>
                        <ModalCloseButton />

                        <ModalBody pb={4} overflowY="auto" flex="1" minH="0">
                            <VStack spacing={6} align="stretch">
                                <Box p={4} borderWidth="1px" borderRadius="md" bg="gray.50">
                                    <Heading size="sm" mb={4}>Header Information</Heading>
                                    <HStack spacing={4} align="flex-start" mb={4}>
                                        <FormControl isInvalid={!!errors.code} isRequired>
                                            <FormLabel>BOM Code</FormLabel>
                                            <Input {...register('code')} placeholder="e.g. BOM-001" bg="white" />
                                            <FormErrorMessage>{errors.code?.message}</FormErrorMessage>
                                        </FormControl>

                                        <FormControl isInvalid={!!errors.name} isRequired>
                                            <FormLabel>BOM Name</FormLabel>
                                            <Input {...register('name')} placeholder="e.g. Lapangan Padel Standard" bg="white" />
                                            <FormErrorMessage>{errors.name?.message}</FormErrorMessage>
                                        </FormControl>
                                    </HStack>

                                    <HStack spacing={4} align="flex-start" mb={4}>
                                        <FormControl isInvalid={!!errors.target_product_id} isRequired flex={2}>
                                            <FormLabel>Target Assembly Product</FormLabel>
                                            <Controller
                                                control={control}
                                                name="target_product_id"
                                                render={({ field }) => (
                                                    <SearchableSelect
                                                        options={productOptions}
                                                        value={field.value ? field.value.toString() : ''}
                                                        onChange={(val) => field.onChange(typeof val === 'string' ? parseInt(val) : val)}
                                                        placeholder="Search product..."
                                                    />
                                                )}
                                            />
                                            <FormErrorMessage>{errors.target_product_id?.message}</FormErrorMessage>
                                        </FormControl>

                                        <FormControl isInvalid={!!errors.target_quantity} isRequired flex={1}>
                                            <FormLabel>Target Qty</FormLabel>
                                            <Controller
                                                control={control}
                                                name="target_quantity"
                                                render={({ field }) => (
                                                    <NumberInput
                                                        min={1}
                                                        value={Number.isNaN(field.value) ? '' : field.value}
                                                        onChange={(_, val) => field.onChange(val)}
                                                        bg="white"
                                                    >
                                                        <NumberInputField />
                                                    </NumberInput>
                                                )}
                                            />
                                            <FormErrorMessage>{errors.target_quantity?.message}</FormErrorMessage>
                                        </FormControl>
                                    </HStack>

                                    <FormControl isInvalid={!!errors.description}>
                                        <FormLabel>Description / Notes</FormLabel>
                                        <Textarea {...register('description')} rows={2} bg="white" />
                                    </FormControl>
                                </Box>

                                <Divider />

                                <Box>
                                    <Flex justify="space-between" align="center" mb={4}>
                                        <Heading size="sm">Components (Base Cost Calculator)</Heading>
                                        <Button
                                            leftIcon={<FiPlus />}
                                            size="sm"
                                            colorScheme="blue"
                                            variant="outline"
                                            onClick={() => append({
                                                component_product_id: 0,
                                                category_str: '',
                                                base_quantity: 1,
                                                base_unit: '',
                                                notes: '',
                                                unit_price: 0,
                                                requirement_quantity: 1,
                                            })}
                                        >
                                            Add Component
                                        </Button>
                                    </Flex>

                                    <Box overflowX="auto" overflowY="auto" maxH="350px" borderWidth="1px" borderRadius="md">
                                        <Table size="sm" variant="simple">
                                            <Thead bg="gray.100" position="sticky" top={0} zIndex={1}>
                                                <Tr>
                                                    <Th minW="160px">Material</Th>
                                                    <Th minW="110px">Category</Th>
                                                    <Th minW="120px">Stok / Unit</Th>
                                                    <Th minW="100px">Notes</Th>
                                                    <Th minW="70px" isNumeric>Req Qty</Th>
                                                    <Th minW="120px" isNumeric>Unit Price</Th>
                                                    <Th minW="120px" isNumeric>Total Cost</Th>
                                                    <Th w="40px"></Th>
                                                </Tr>
                                            </Thead>
                                            <Tbody>
                                                {fields.map((field, index) => {
                                                    const currentUnitPrice = watchedItems[index]?.unit_price || 0;
                                                    const currentReqQty = watchedItems[index]?.requirement_quantity || 0;
                                                    const lineTotal = currentUnitPrice * currentReqQty;

                                                    return (
                                                        <Tr key={field.id}>
                                                            <Td>
                                                                <Controller
                                                                    control={control}
                                                                    name={`items.${index}.component_product_id`}
                                                                    render={({ field: selectField }) => (
                                                                        <SearchableSelect
                                                                            options={productOptions}
                                                                            value={selectField.value ? selectField.value.toString() : ''}
                                                                            onChange={(val) => {
                                                                                selectField.onChange(typeof val === 'string' ? parseInt(val) : val);
                                                                                handleProductSelect(index, val);
                                                                            }}
                                                                            placeholder="Select material..."
                                                                            size="sm"
                                                                        />
                                                                    )}
                                                                />
                                                            </Td>
                                                            <Td>
                                                                <Input size="sm" {...register(`items.${index}.category_str`)} placeholder="e.g. A. Kelengkapan" />
                                                            </Td>
                                                            <Td>
                                                                {/* Stok / Unit: read-only, auto-filled from product stock */}
                                                                <HStack spacing={1}>
                                                                    <Badge
                                                                        colorScheme={(watchedItems[index]?.base_quantity ?? 0) > 0 ? 'green' : 'red'}
                                                                        variant="subtle"
                                                                        fontSize="sm"
                                                                        px={2} py={1}
                                                                        borderRadius="md"
                                                                        minW="40px"
                                                                        textAlign="center"
                                                                    >
                                                                        {watchedItems[index]?.base_quantity ?? 0}
                                                                    </Badge>
                                                                    <Input size="sm" w="55px" {...register(`items.${index}.base_unit`)} placeholder="pcs" px={1} />
                                                                </HStack>
                                                            </Td>
                                                            <Td>
                                                                <Input size="sm" {...register(`items.${index}.notes`)} placeholder="Keterangan..." />
                                                            </Td>
                                                            <Td isNumeric>
                                                                <Controller
                                                                    control={control}
                                                                    name={`items.${index}.requirement_quantity`}
                                                                    render={({ field: numField }) => (
                                                                        <NumberInput
                                                                            size="sm"
                                                                            min={0.01}
                                                                            value={Number.isNaN(numField.value) ? '' : numField.value}
                                                                            onChange={(_, val) => numField.onChange(val)}
                                                                        >
                                                                            <NumberInputField />
                                                                        </NumberInput>
                                                                    )}
                                                                />
                                                            </Td>
                                                            <Td isNumeric>
                                                                <Text
                                                                    fontSize="sm"
                                                                    fontWeight="medium"
                                                                    color="gray.700"
                                                                    textAlign="right"
                                                                    px={2}
                                                                    py={1}
                                                                    bg="gray.50"
                                                                    borderRadius="md"
                                                                    border="1px solid"
                                                                    borderColor="gray.200"
                                                                >
                                                                    Rp {(watchedItems[index]?.unit_price || 0).toLocaleString('id-ID')}
                                                                </Text>
                                                            </Td>
                                                            <Td isNumeric fontWeight="bold" color="blue.600">
                                                                {new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(lineTotal)}
                                                            </Td>
                                                            <Td>
                                                                <IconButton
                                                                    aria-label="Remove item"
                                                                    icon={<FiTrash2 />}
                                                                    size="sm"
                                                                    colorScheme="red"
                                                                    variant="ghost"
                                                                    onClick={() => remove(index)}
                                                                />
                                                            </Td>
                                                        </Tr>
                                                    );
                                                })}
                                                {fields.length === 0 && (
                                                    <Tr>
                                                        <Td colSpan={8} textAlign="center" py={4} color="gray.500">
                                                            No components added. Click "Add Component" to start building the BOM.
                                                        </Td>
                                                    </Tr>
                                                )}
                                                {!!errors.items && (
                                                    <Tr>
                                                        <Td colSpan={8} textAlign="center" color="red.500" py={2} bg="red.50">
                                                            {errors.items.message as string}
                                                        </Td>
                                                    </Tr>
                                                )}
                                            </Tbody>
                                            <Tfoot position="sticky" bottom={0} zIndex={1}>
                                                <Tr bg="blue.50">
                                                    <Td colSpan={6} textAlign="right" fontWeight="bold" fontSize="sm" color="gray.700" borderTop="2px" borderColor="blue.200">
                                                        Total Unit Cost (per 1 unit):
                                                    </Td>
                                                    <Td isNumeric fontWeight="bold" fontSize="md" color="blue.700" borderTop="2px" borderColor="blue.200">
                                                        {new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(calculateTotal())}
                                                    </Td>
                                                    <Td borderTop="2px" borderColor="blue.200" />
                                                </Tr>
                                                {watchedTargetQty > 1 && (
                                                    <Tr bg="orange.50">
                                                        <Td colSpan={6} textAlign="right" fontWeight="bold" fontSize="sm" color="orange.700" borderTop="1px" borderColor="orange.200">
                                                            Total Batch Cost (× {watchedTargetQty} units):
                                                        </Td>
                                                        <Td isNumeric fontWeight="bold" fontSize="md" color="orange.700" borderTop="1px" borderColor="orange.200">
                                                            {new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(calculateBatchTotal())}
                                                        </Td>
                                                        <Td borderTop="1px" borderColor="orange.200" />
                                                    </Tr>
                                                )}
                                            </Tfoot>
                                        </Table>
                                    </Box>
                                </Box>
                            </VStack>
                        </ModalBody>

                        <ModalFooter borderTopWidth="1px" bg="gray.50">
                            <Button variant="ghost" mr={3} onClick={onClose} isDisabled={isSubmitting}>
                                Cancel
                            </Button>
                            <Button
                                colorScheme="blue"
                                type="submit"
                                isLoading={isSubmitting}
                            >
                                Save BOM Template
                            </Button>
                        </ModalFooter>
                    </form>
                )}
            </ModalContent>
        </Modal>
    );
};

export default BOMFormModal;
