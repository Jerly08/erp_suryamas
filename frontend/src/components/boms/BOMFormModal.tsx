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
    Checkbox,
    useColorModeValue,
} from '@chakra-ui/react';
import { FiPlus, FiTrash2, FiClipboard } from 'react-icons/fi';
import { useForm, Controller, useFieldArray } from 'react-hook-form';
import { zodResolver } from '@hookform/resolvers/zod';
import * as z from 'zod';

import { bomService, componentTemplateService } from '@/services/bomService';
import ProductService, { Product } from '@/services/productService';
import { BOM } from '@/types/bom';
import SearchableSelect from '@/components/common/SearchableSelect';
import UseComponentTemplateModal from './UseComponentTemplateModal';

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
    const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const tableHeadBg = useColorModeValue('gray.100', 'var(--bg-tertiary)');
    const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
    const headingColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const labelColor = useColorModeValue('gray.500', 'var(--text-secondary)');
    const textColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const noteColor = useColorModeValue('gray.600', 'var(--text-secondary)');
    const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.50');
    const footerBg = useColorModeValue('blue.50', 'whiteAlpha.100');
    const footerBorderColor = useColorModeValue('blue.200', 'blue.400');
    const costColor = useColorModeValue('blue.600', 'blue.300');

    return (
        <VStack spacing={5} align="stretch">
            {/* Header Box */}
            <Box p={4} borderWidth="1px" borderRadius="md" bg={panelBg} borderColor={borderColor}>
                <Heading size="sm" mb={4} color={headingColor}>Header Information</Heading>
                <SimpleGrid columns={2} spacing={4} mb={4}>
                    <Box>
                        <Text fontSize="xs" color={labelColor} fontWeight="semibold" mb={1}>BOM CODE</Text>
                        <Text fontWeight="bold" fontSize="md" color={textColor}>{bom.code}</Text>
                    </Box>
                    <Box>
                        <Text fontSize="xs" color={labelColor} fontWeight="semibold" mb={1}>BOM NAME</Text>
                        <Text fontWeight="bold" fontSize="md" color={textColor}>{bom.name}</Text>
                    </Box>
                    <Box>
                        <Text fontSize="xs" color={labelColor} fontWeight="semibold" mb={1}>TARGET ASSEMBLY PRODUCT</Text>
                        <Text fontSize="md" color={textColor}>{bom.target_product?.name || `Product ID: ${bom.target_product_id}`}</Text>
                    </Box>
                    <Box>
                        <Text fontSize="xs" color={labelColor} fontWeight="semibold" mb={1}>TARGET QUANTITY</Text>
                        <Text fontSize="md" color={textColor}>{bom.target_quantity}</Text>
                    </Box>
                </SimpleGrid>
                {bom.description && (
                    <Box>
                        <Text fontSize="xs" color={labelColor} fontWeight="semibold" mb={1}>DESCRIPTION / NOTES</Text>
                        <Text fontSize="sm" color={textColor} whiteSpace="pre-wrap">{bom.description}</Text>
                    </Box>
                )}
            </Box>

            <Divider />

            {/* Components Table */}
            <Box>
                <Flex justify="space-between" align="center" mb={3}>
                    <Heading size="sm" color={headingColor}>Components (Base Cost Calculator)</Heading>
                    <Badge colorScheme="purple" variant="subtle" fontSize="sm" px={3} py={1} borderRadius="full">
                        {bom.items?.length || 0} Components
                    </Badge>
                </Flex>

                <Box overflowX="auto" borderWidth="1px" borderRadius="md" borderColor={borderColor}>
                    <Table size="sm" variant="simple">
                        <Thead bg={tableHeadBg}>
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
                                        <Tr key={item.id ?? idx} _hover={{ bg: rowHoverBg }}>
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
                                                <Text fontSize="sm" color={textColor}>
                                                    {item.component_product?.stock ?? item.base_quantity} {item.base_unit || ''}
                                                </Text>
                                            </Td>
                                            <Td>
                                                <Text fontSize="xs" color={noteColor}>{item.notes || '—'}</Text>
                                            </Td>
                                            <Td isNumeric>{item.requirement_quantity}</Td>
                                            <Td isNumeric>{formatCurrency(item.unit_price || 0)}</Td>
                                            <Td isNumeric fontWeight="bold" color={costColor}>
                                                {formatCurrency(lineTotal)}
                                            </Td>
                                        </Tr>
                                    );
                                })
                            ) : (
                                <Tr>
                                    <Td colSpan={8} textAlign="center" py={4} color={labelColor}>
                                        No components found.
                                    </Td>
                                </Tr>
                            )}
                        </Tbody>
                        <Tfoot>
                            <Tr bg={footerBg}>
                                <Td colSpan={6} textAlign="right" fontWeight="bold" fontSize="sm" color={headingColor} borderTop="1px" borderColor={footerBorderColor}>
                                    Total Unit Cost:
                                </Td>
                                <Td isNumeric fontWeight="bold" fontSize="md" color={costColor} borderTop="1px" borderColor={footerBorderColor}>
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
    const [saveAsTemplate, setSaveAsTemplate] = useState(false);
    const [templateName, setTemplateName] = useState('');
    const [useTemplateOpen, setUseTemplateOpen] = useState(false);
    const [existingBOMProductIds, setExistingBOMProductIds] = useState<number[]>([]);
    const toast = useToast();
    const modalBg = useColorModeValue('white', 'var(--bg-secondary)');
    const panelBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const sectionBg = useColorModeValue('green.50', 'whiteAlpha.100');
    const sectionBorderColor = useColorModeValue('gray.200', 'var(--border-color)');
    const textColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const labelColor = useColorModeValue('gray.600', 'var(--text-secondary)');
    const inputBg = useColorModeValue('white', 'var(--bg-secondary)');
    const inputBorderColor = useColorModeValue('gray.300', 'var(--border-color)');
    const hoverBorderColor = useColorModeValue('gray.400', 'gray.500');
    const tableHeadBg = useColorModeValue('gray.100', 'var(--bg-tertiary)');
    const rowHoverBg = useColorModeValue('gray.50', 'whiteAlpha.50');
    const footerBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const costBg = useColorModeValue('gray.50', 'whiteAlpha.100');
    const costBorderColor = useColorModeValue('gray.200', 'var(--border-color)');
    const costTextColor = useColorModeValue('gray.700', 'gray.100');
    const totalFooterBg = useColorModeValue('blue.50', 'whiteAlpha.100');
    const totalFooterBorderColor = useColorModeValue('blue.200', 'blue.400');
    const totalTextColor = useColorModeValue('blue.700', 'blue.300');
    const batchFooterBg = useColorModeValue('orange.50', 'whiteAlpha.100');
    const batchFooterBorderColor = useColorModeValue('orange.200', 'orange.400');
    const batchTextColor = useColorModeValue('orange.700', 'orange.300');
    const checkboxLabelColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const errorRowBg = useColorModeValue('red.50', 'rgba(245, 101, 101, 0.12)');

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
    const watchedBomName = watch('name');
    const watchedBomCode = watch('code');
    const getDefaultTemplateName = (code?: string, name?: string) => {
        const safeCode = code?.trim() || '';
        const safeName = name?.trim() || '';

        if (safeCode && safeName) {
            return `${safeCode} - ${safeName}`;
        }

        return safeName || safeCode;
    };
    const componentProductOptions = products
        .filter((product) => product.is_assembly !== true)
        .map((product) => ({
            id: product.id || 0,
            name: `${product.code} - ${product.name}`,
        }));

    useEffect(() => {
        if (!isViewOnly) {
            fetchProducts();
        }
    }, [isViewOnly]);

    useEffect(() => {
        if (isOpen && !isViewOnly) {
            setSaveAsTemplate(false);
            setTemplateName('');
        }

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

    useEffect(() => {
        if (!saveAsTemplate) {
            return;
        }

        if (templateName.trim()) {
            return;
        }

        const fallbackName = getDefaultTemplateName(watchedBomCode, watchedBomName);
        if (fallbackName) {
            setTemplateName(fallbackName);
        }
    }, [saveAsTemplate, templateName, watchedBomName, watchedBomCode]);

    const fetchProducts = async () => {
        try {
            const [productResponse, bomResponse] = await Promise.all([
                ProductService.getProducts({ limit: 1000 }),
                bomService.getBOMs(),
            ]);
            if (productResponse.data) {
                setProducts(productResponse.data);
            }
            if (bomResponse.data) {
                setExistingBOMProductIds(bomResponse.data.map((b: any) => b.target_product_id));
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

    const handleLoadTemplate = (templateItems: { component_product_id: number; category_str: string; requirement_quantity: number; base_unit: string; notes: string }[]) => {
        for (const item of templateItems) {
            const product = products.find((p) => p.id === item.component_product_id);
            append({
                component_product_id: item.component_product_id,
                category_str: item.category_str,
                base_quantity: product?.stock ?? 0,
                base_unit: item.base_unit || product?.unit || '',
                notes: item.notes || '',
                unit_price: product?.purchase_price || 0,
                requirement_quantity: item.requirement_quantity,
            });
        }
        toast({ title: `${templateItems.length} komponen dimuat dari template`, status: 'info', isClosable: true });
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
                toast({ title: 'BOM updated successfully', status: 'success', isClosable: true });
            } else {
                await bomService.createBOM(payload);
                toast({ title: 'BOM created successfully', status: 'success', isClosable: true });
            }

            // Save as template if checkbox is checked
            if (saveAsTemplate && data.items.length > 0) {
                const resolvedTemplateName = templateName.trim() || getDefaultTemplateName(data.code, data.name);
                const templatePayload = {
                    name: resolvedTemplateName,
                    description: data.description || '',
                    items: data.items.map(item => ({
                        component_product_id: item.component_product_id,
                        category_str: item.category_str,
                        requirement_quantity: item.requirement_quantity,
                        base_unit: item.base_unit,
                        notes: item.notes || '',
                    })),
                };
                try {
                    const existingTemplates = await componentTemplateService.getTemplates();
                    const matchedTemplate = existingTemplates.data?.find((template) =>
                        template.name.trim().toLowerCase() === resolvedTemplateName.trim().toLowerCase()
                    );

                    if (matchedTemplate?.id) {
                        await componentTemplateService.updateTemplate(matchedTemplate.id, templatePayload);
                    } else {
                        await componentTemplateService.createTemplate(templatePayload);
                    }

                    toast({
                        title: 'Component template tersimpan',
                        description: resolvedTemplateName,
                        status: 'success',
                        isClosable: true
                    });
                } catch (tplError: any) {
                    toast({ title: 'Template gagal disimpan', description: tplError.message, status: 'warning', isClosable: true });
                }
            }

            onSuccess();
            onClose();
        } catch (error: any) {
            toast({ title: 'Failed to save BOM', description: error.message, status: 'error', isClosable: true });
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
            <ModalContent my={4} maxH="90vh" bg={modalBg} border="1px solid" borderColor={sectionBorderColor}>
                {/* ===== VIEW ONLY MODE ===== */}
                {isViewOnly && existingBOM ? (
                    <>
                        <ModalHeader borderBottomWidth="1px" borderColor={sectionBorderColor} color={textColor}>
                            <HStack spacing={3}>
                                <Text>View BOM Template</Text>
                                <Badge colorScheme="gray" variant="outline">{existingBOM.code}</Badge>
                            </HStack>
                        </ModalHeader>
                        <ModalCloseButton />
                        <ModalBody pb={6}>
                            <BOMViewContent bom={existingBOM} />
                        </ModalBody>
                        <ModalFooter borderTopWidth="1px" borderColor={sectionBorderColor} bg={footerBg}>
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
                        <ModalHeader color={textColor}>
                            {existingBOM ? 'Edit BOM Template' : 'Create New BOM Template'}
                        </ModalHeader>
                        <ModalCloseButton />

                        <ModalBody pb={4} overflowY="auto" flex="1" minH="0">
                            <VStack spacing={6} align="stretch">
                                <Box p={4} borderWidth="1px" borderRadius="md" bg={panelBg} borderColor={sectionBorderColor}>
                                    <Heading size="sm" mb={4} color={textColor}>Header Information</Heading>
                                    <HStack spacing={4} align="flex-start" mb={4}>
                                        <FormControl isInvalid={!!errors.code} isRequired>
                                            <FormLabel color={labelColor}>BOM Code</FormLabel>
                                            <Input {...register('code')} placeholder="e.g. BOM-001" bg={inputBg} color={textColor} borderColor={inputBorderColor} _hover={{ borderColor: hoverBorderColor }} _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }} />
                                            <FormErrorMessage>{errors.code?.message}</FormErrorMessage>
                                        </FormControl>

                                        <FormControl isInvalid={!!errors.name} isRequired>
                                            <FormLabel color={labelColor}>BOM Name</FormLabel>
                                            <Input {...register('name')} placeholder="e.g. Lapangan Padel Standard" bg={inputBg} color={textColor} borderColor={inputBorderColor} _hover={{ borderColor: hoverBorderColor }} _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }} />
                                            <FormErrorMessage>{errors.name?.message}</FormErrorMessage>
                                        </FormControl>
                                    </HStack>

                                    <HStack spacing={4} align="flex-start" mb={4}>
                                        <FormControl isInvalid={!!errors.target_product_id} isRequired flex={2}>
                                            <FormLabel color={labelColor}>Target Assembly Product</FormLabel>
                                            <Controller
                                                control={control}
                                                name="target_product_id"
                                                render={({ field }) => {
                                                    // Only assembly products can become BOM targets,
                                                    // and each target product should only have one BOM.
                                                    const filteredTargetOptions = products
                                                        .filter((product) => {
                                                            const productId = product.id || 0;
                                                            if (existingBOM && productId === existingBOM.target_product_id) return true;
                                                            return product.is_assembly === true && !existingBOMProductIds.includes(productId);
                                                        })
                                                        .map((product) => ({
                                                            id: product.id || 0,
                                                            name: `${product.code} - ${product.name}`,
                                                        }));
                                                    return (
                                                        <SearchableSelect
                                                            options={filteredTargetOptions}
                                                            value={field.value ? field.value.toString() : ''}
                                                            onChange={(val) => field.onChange(typeof val === 'string' ? parseInt(val) : val)}
                                                            placeholder="Search product..."
                                                        />
                                                    );
                                                }}
                                            />
                                            <FormErrorMessage>{errors.target_product_id?.message}</FormErrorMessage>
                                        </FormControl>

                                        <FormControl isInvalid={!!errors.target_quantity} isRequired flex={1}>
                                            <FormLabel color={labelColor}>Target Qty</FormLabel>
                                            <Controller
                                                control={control}
                                                name="target_quantity"
                                                render={({ field }) => (
                                                    <NumberInput
                                                        min={1}
                                                        value={Number.isNaN(field.value) ? '' : field.value}
                                                        onChange={(_, val) => field.onChange(val)}
                                                        bg={inputBg}
                                                    >
                                                        <NumberInputField />
                                                    </NumberInput>
                                                )}
                                            />
                                            <FormErrorMessage>{errors.target_quantity?.message}</FormErrorMessage>
                                        </FormControl>
                                    </HStack>

                                    <FormControl isInvalid={!!errors.description}>
                                        <FormLabel color={labelColor}>Description / Notes</FormLabel>
                                        <Textarea {...register('description')} rows={2} bg={inputBg} color={textColor} borderColor={inputBorderColor} _hover={{ borderColor: hoverBorderColor }} _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }} />
                                    </FormControl>
                                </Box>

                                <Divider />

                                <Box>
                                    <Flex justify="space-between" align="center" mb={4}>
                                        <Heading size="sm" color={textColor}>Components (Base Cost Calculator)</Heading>
                                        <HStack spacing={2}>
                                            <Button
                                                leftIcon={<FiClipboard />}
                                                size="sm"
                                                colorScheme="green"
                                                variant="outline"
                                                onClick={() => setUseTemplateOpen(true)}
                                            >
                                                Use Template
                                            </Button>
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
                                        </HStack>
                                    </Flex>

                                    <Box overflowX="auto" overflowY="auto" maxH="350px" borderWidth="1px" borderRadius="md" borderColor={sectionBorderColor}>
                                        <Table size="sm" variant="simple">
                                            <Thead bg={tableHeadBg} position="sticky" top={0} zIndex={1}>
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
                                                        <Tr key={field.id} _hover={{ bg: rowHoverBg }}>
                                                            <Td>
                                                                <Controller
                                                                    control={control}
                                                                    name={`items.${index}.component_product_id`}
                                                                    render={({ field: selectField }) => (
                                                                        <SearchableSelect
                                                                            options={componentProductOptions}
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
                                                                <Input size="sm" {...register(`items.${index}.category_str`)} placeholder="e.g. A. Kelengkapan" bg={inputBg} color={textColor} borderColor={inputBorderColor} _hover={{ borderColor: hoverBorderColor }} _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }} />
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
                                                                    <Input size="sm" w="55px" {...register(`items.${index}.base_unit`)} placeholder="pcs" px={1} bg={inputBg} color={textColor} borderColor={inputBorderColor} _hover={{ borderColor: hoverBorderColor }} _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }} />
                                                                </HStack>
                                                            </Td>
                                                            <Td>
                                                                <Input size="sm" {...register(`items.${index}.notes`)} placeholder="Keterangan..." bg={inputBg} color={textColor} borderColor={inputBorderColor} _hover={{ borderColor: hoverBorderColor }} _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }} />
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
                                                                    color={costTextColor}
                                                                    textAlign="right"
                                                                    px={2}
                                                                    py={1}
                                                                    bg={costBg}
                                                                    borderRadius="md"
                                                                    border="1px solid"
                                                                    borderColor={costBorderColor}
                                                                >
                                                                    Rp {(watchedItems[index]?.unit_price || 0).toLocaleString('id-ID')}
                                                                </Text>
                                                            </Td>
                                                            <Td isNumeric fontWeight="bold" color={totalTextColor}>
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
                                                        <Td colSpan={8} textAlign="center" py={4} color={labelColor}>
                                                            No components added. Click &quot;Add Component&quot; to start building the BOM.
                                                        </Td>
                                                    </Tr>
                                                )}
                                                {!!errors.items && (
                                                    <Tr>
                                                        <Td colSpan={8} textAlign="center" color="red.500" py={2} bg={errorRowBg}>
                                                            {errors.items.message as string}
                                                        </Td>
                                                    </Tr>
                                                )}
                                            </Tbody>
                                            <Tfoot position="sticky" bottom={0} zIndex={1}>
                                                <Tr bg={totalFooterBg}>
                                                    <Td colSpan={6} textAlign="right" fontWeight="bold" fontSize="sm" color={textColor} borderTop="2px" borderColor={totalFooterBorderColor}>
                                                        Total Unit Cost (per 1 unit):
                                                    </Td>
                                                    <Td isNumeric fontWeight="bold" fontSize="md" color={totalTextColor} borderTop="2px" borderColor={totalFooterBorderColor}>
                                                        {new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(calculateTotal())}
                                                    </Td>
                                                    <Td borderTop="2px" borderColor={totalFooterBorderColor} />
                                                </Tr>
                                                {watchedTargetQty > 1 && (
                                                    <Tr bg={batchFooterBg}>
                                                        <Td colSpan={6} textAlign="right" fontWeight="bold" fontSize="sm" color={batchTextColor} borderTop="1px" borderColor={batchFooterBorderColor}>
                                                            Total Batch Cost (× {watchedTargetQty} units):
                                                        </Td>
                                                        <Td isNumeric fontWeight="bold" fontSize="md" color={batchTextColor} borderTop="1px" borderColor={batchFooterBorderColor}>
                                                            {new Intl.NumberFormat('id-ID', { style: 'currency', currency: 'IDR' }).format(calculateBatchTotal())}
                                                        </Td>
                                                        <Td borderTop="1px" borderColor={batchFooterBorderColor} />
                                                    </Tr>
                                                )}
                                            </Tfoot>
                                        </Table>
                                    </Box>

                                    {/* Save as Template checkbox */}
                                    <Box mt={4} p={3} borderWidth="1px" borderRadius="md" bg={sectionBg} borderColor={sectionBorderColor}>
                                        <Flex align="center" gap={3}>
                                            <Checkbox
                                                id="saveAsTemplate"
                                                isChecked={saveAsTemplate}
                                                colorScheme="green"
                                                onChange={(e) => {
                                                    const checked = e.target.checked;
                                                    setSaveAsTemplate(checked);
                                                    if (!checked) {
                                                        setTemplateName('');
                                                    } else if (!templateName.trim()) {
                                                        setTemplateName(getDefaultTemplateName(watchedBomCode, watchedBomName));
                                                    }
                                                }}
                                            >
                                                <Text fontSize="sm" fontWeight="medium" color={checkboxLabelColor}>
                                                    Save components as template
                                                </Text>
                                            </Checkbox>
                                        </Flex>
                                        {saveAsTemplate && (
                                            <Box mt={2}>
                                                <Input
                                                    size="sm"
                                                    placeholder="Nama template, e.g. Paket Lapangan Padel Standard"
                                                    value={templateName}
                                                    onChange={(e) => setTemplateName(e.target.value)}
                                                    bg={inputBg}
                                                    color={textColor}
                                                    borderColor={inputBorderColor}
                                                    _hover={{ borderColor: hoverBorderColor }}
                                                    _focusVisible={{ borderColor: 'blue.400', boxShadow: '0 0 0 1px var(--chakra-colors-blue-400)' }}
                                                />
                                            </Box>
                                        )}
                                    </Box>
                                </Box>
                            </VStack>

                            {/* Use Template Modal */}
                            <UseComponentTemplateModal
                                isOpen={useTemplateOpen}
                                onClose={() => setUseTemplateOpen(false)}
                                onLoad={handleLoadTemplate}
                            />
                        </ModalBody>

                        <ModalFooter borderTopWidth="1px" borderColor={sectionBorderColor} bg={footerBg}>
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
