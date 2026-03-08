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
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    Text,
    IconButton,
    useToast,
    Box,
    Badge,
    Flex,
    Spinner,
    AlertDialog,
    AlertDialogOverlay,
    AlertDialogContent,
    AlertDialogHeader,
    AlertDialogBody,
    AlertDialogFooter,
    Accordion,
    AccordionItem,
    AccordionButton,
    AccordionPanel,
    AccordionIcon,
    HStack,
    VStack,
    Input,
    FormControl,
    FormLabel,
} from '@chakra-ui/react';
import { FiTrash2, FiPlus, FiSave, FiX, FiEdit } from 'react-icons/fi';
import { componentTemplateService } from '@/services/bomService';
import { ComponentTemplate } from '@/types/bom';
import ProductService, { Product } from '@/services/productService';
import SearchableSelect from '@/components/common/SearchableSelect';

interface ComponentTemplateManagerModalProps {
    isOpen: boolean;
    onClose: () => void;
}

interface EditableItem {
    component_product_id: number;
    category_str: string;
    requirement_quantity: number;
    base_unit: string;
    notes: string;
}

const ComponentTemplateManagerModal: React.FC<ComponentTemplateManagerModalProps> = ({ isOpen, onClose }) => {
    const [templates, setTemplates] = useState<ComponentTemplate[]>([]);
    const [loading, setLoading] = useState(false);
    const [deleteId, setDeleteId] = useState<number | null>(null);
    const cancelRef = React.useRef<HTMLButtonElement>(null);
    const toast = useToast();

    // Form state (shared for create and edit)
    const [formMode, setFormMode] = useState<'none' | 'create' | 'edit'>('none');
    const [editingId, setEditingId] = useState<number | null>(null);
    const [formName, setFormName] = useState('');
    const [formDescription, setFormDescription] = useState('');
    const [formItems, setFormItems] = useState<EditableItem[]>([]);
    const [products, setProducts] = useState<Product[]>([]);
    const [productOptions, setProductOptions] = useState<{ id: string | number; name: string }[]>([]);
    const [saving, setSaving] = useState(false);

    const fetchTemplates = async () => {
        setLoading(true);
        try {
            const response = await componentTemplateService.getTemplates();
            if (response.data) {
                setTemplates(response.data);
            }
        } catch (error) {
            console.error('Failed to fetch templates', error);
        } finally {
            setLoading(false);
        }
    };

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

    useEffect(() => {
        if (isOpen) {
            fetchTemplates();
            fetchProducts();
            resetForm();
        }
    }, [isOpen]);

    const handleDelete = async () => {
        if (deleteId === null) return;
        try {
            await componentTemplateService.deleteTemplate(deleteId);
            toast({ title: 'Template berhasil dihapus', status: 'success', isClosable: true });
            if (editingId === deleteId) resetForm();
            fetchTemplates();
        } catch (error: any) {
            toast({ title: 'Gagal menghapus template', description: error.message, status: 'error', isClosable: true });
        } finally {
            setDeleteId(null);
        }
    };

    // --- Form handlers ---
    const resetForm = () => {
        setFormMode('none');
        setEditingId(null);
        setFormName('');
        setFormDescription('');
        setFormItems([]);
    };

    const handleStartCreate = () => {
        resetForm();
        setFormMode('create');
    };

    const handleStartEdit = (tpl: ComponentTemplate) => {
        setFormMode('edit');
        setEditingId(tpl.id);
        setFormName(tpl.name);
        setFormDescription(tpl.description || '');
        setFormItems(
            (tpl.items || []).map((item) => ({
                component_product_id: item.component_product_id,
                category_str: item.category_str,
                requirement_quantity: item.requirement_quantity,
                base_unit: item.base_unit,
                notes: item.notes || '',
            }))
        );
    };

    const handleAddItem = () => {
        setFormItems([...formItems, { component_product_id: 0, category_str: '', requirement_quantity: 1, base_unit: '', notes: '' }]);
    };

    const handleRemoveItem = (index: number) => {
        setFormItems(formItems.filter((_, i) => i !== index));
    };

    const handleItemChange = (index: number, field: keyof EditableItem, value: any) => {
        const updated = [...formItems];
        (updated[index] as any)[field] = value;

        if (field === 'component_product_id') {
            const product = products.find((p) => p.id === value);
            if (product) {
                updated[index].base_unit = product.unit || '';
                updated[index].category_str = product.category?.name || '';
            }
        }

        setFormItems(updated);
    };

    const handleSave = async () => {
        if (!formName.trim()) {
            toast({ title: 'Nama template wajib diisi', status: 'warning', isClosable: true });
            return;
        }
        const validItems = formItems.filter((item) => item.component_product_id > 0 && item.requirement_quantity > 0);
        if (validItems.length === 0) {
            toast({ title: 'Tambahkan minimal 1 komponen', status: 'warning', isClosable: true });
            return;
        }

        setSaving(true);
        try {
            const payload = {
                name: formName.trim(),
                description: formDescription.trim(),
                items: validItems,
            };

            if (formMode === 'edit' && editingId) {
                await componentTemplateService.updateTemplate(editingId, payload);
                toast({ title: 'Template berhasil diperbarui!', status: 'success', isClosable: true });
            } else {
                await componentTemplateService.createTemplate(payload);
                toast({ title: 'Template berhasil disimpan!', status: 'success', isClosable: true });
            }

            resetForm();
            fetchTemplates();
        } catch (error: any) {
            toast({ title: 'Gagal menyimpan template', description: error.response?.data?.error || error.message, status: 'error', isClosable: true });
        } finally {
            setSaving(false);
        }
    };

    const formatDate = (dateStr: string) => {
        try {
            return new Date(dateStr).toLocaleDateString('id-ID', { day: '2-digit', month: 'short', year: 'numeric' });
        } catch {
            return dateStr;
        }
    };

    // === Shared form component ===
    const renderForm = () => (
        <Box mb={5} p={4} borderWidth="2px" borderRadius="md" borderColor={formMode === 'edit' ? 'orange.300' : 'blue.300'} bg={formMode === 'edit' ? 'orange.50' : 'blue.50'}>
            <Text fontWeight="bold" fontSize="md" mb={3} color={formMode === 'edit' ? 'orange.700' : 'blue.700'}>
                {formMode === 'edit' ? `Edit Template: ${formName}` : 'Buat Template Baru'}
            </Text>

            <HStack spacing={4} mb={3}>
                <FormControl isRequired flex={2}>
                    <FormLabel fontSize="sm">Nama Template</FormLabel>
                    <Input size="sm" bg="white" placeholder="e.g. Paket Lapangan Padel Standard" value={formName} onChange={(e) => setFormName(e.target.value)} />
                </FormControl>
                <FormControl flex={3}>
                    <FormLabel fontSize="sm">Deskripsi</FormLabel>
                    <Input size="sm" bg="white" placeholder="Opsional" value={formDescription} onChange={(e) => setFormDescription(e.target.value)} />
                </FormControl>
            </HStack>

            <Flex justify="space-between" align="center" mb={2}>
                <Text fontSize="sm" fontWeight="semibold" color="gray.700">Komponen:</Text>
                <Button leftIcon={<FiPlus />} size="xs" colorScheme="blue" variant="outline" onClick={handleAddItem}>Add Component</Button>
            </Flex>

            {formItems.length === 0 ? (
                <Box textAlign="center" py={3} bg="white" borderRadius="md" borderWidth="1px">
                    <Text fontSize="sm" color="gray.400">Belum ada komponen. Klik &quot;Add Component&quot; untuk menambah.</Text>
                </Box>
            ) : (
                <Box overflowX="auto" borderWidth="1px" borderRadius="md" bg="white">
                    <Table size="sm" variant="simple">
                        <Thead bg="gray.50">
                            <Tr>
                                <Th w="35%">Material</Th>
                                <Th w="15%">Category</Th>
                                <Th w="12%" isNumeric>Req Qty</Th>
                                <Th w="12%">Unit</Th>
                                <Th w="20%">Notes</Th>
                                <Th w="6%"></Th>
                            </Tr>
                        </Thead>
                        <Tbody>
                            {formItems.map((item, idx) => (
                                <Tr key={idx}>
                                    <Td>
                                        <SearchableSelect
                                            options={productOptions}
                                            value={item.component_product_id ? item.component_product_id.toString() : ''}
                                            onChange={(val) => handleItemChange(idx, 'component_product_id', typeof val === 'string' ? parseInt(val) : val)}
                                            placeholder="Select material..."
                                        />
                                    </Td>
                                    <Td>
                                        <Input size="sm" value={item.category_str} onChange={(e) => handleItemChange(idx, 'category_str', e.target.value)} placeholder="Category" />
                                    </Td>
                                    <Td>
                                        <Input size="sm" type="number" min={1} value={item.requirement_quantity} onChange={(e) => handleItemChange(idx, 'requirement_quantity', parseFloat(e.target.value) || 0)} textAlign="right" />
                                    </Td>
                                    <Td>
                                        <Input size="sm" value={item.base_unit} onChange={(e) => handleItemChange(idx, 'base_unit', e.target.value)} placeholder="pcs" />
                                    </Td>
                                    <Td>
                                        <Input size="sm" value={item.notes} onChange={(e) => handleItemChange(idx, 'notes', e.target.value)} placeholder="Notes" />
                                    </Td>
                                    <Td>
                                        <IconButton aria-label="Remove" icon={<FiTrash2 />} size="xs" colorScheme="red" variant="ghost" onClick={() => handleRemoveItem(idx)} />
                                    </Td>
                                </Tr>
                            ))}
                        </Tbody>
                    </Table>
                </Box>
            )}

            <HStack mt={3} justify="flex-end" spacing={2}>
                <Button size="sm" leftIcon={<FiX />} variant="ghost" onClick={resetForm}>Batal</Button>
                <Button
                    size="sm"
                    leftIcon={<FiSave />}
                    colorScheme={formMode === 'edit' ? 'orange' : 'blue'}
                    onClick={handleSave}
                    isLoading={saving}
                    isDisabled={!formName.trim() || formItems.length === 0}
                >
                    {formMode === 'edit' ? 'Update Template' : 'Simpan Template'}
                </Button>
            </HStack>
        </Box>
    );

    return (
        <>
            <Modal isOpen={isOpen} onClose={onClose} size="5xl" scrollBehavior="inside">
                <ModalOverlay />
                <ModalContent maxH="90vh">
                    <ModalHeader borderBottomWidth="1px">
                        <Flex justify="space-between" align="center" pr={8}>
                            <Text>Kelola Component Templates</Text>
                            {formMode === 'none' && (
                                <Button leftIcon={<FiPlus />} size="sm" colorScheme="blue" onClick={handleStartCreate}>
                                    Buat Template Baru
                                </Button>
                            )}
                        </Flex>
                    </ModalHeader>
                    <ModalCloseButton />
                    <ModalBody pb={4}>
                        {/* === CREATE / EDIT FORM === */}
                        {formMode !== 'none' && renderForm()}

                        {/* === EXISTING TEMPLATES LIST === */}
                        {loading ? (
                            <Flex justify="center" py={10}><Spinner size="lg" color="blue.500" /></Flex>
                        ) : templates.length === 0 && formMode === 'none' ? (
                            <Box textAlign="center" py={10}>
                                <Text color="gray.500" fontSize="md" mb={3}>Belum ada template tersimpan.</Text>
                                <Button leftIcon={<FiPlus />} colorScheme="blue" size="sm" onClick={handleStartCreate}>Buat Template Pertama</Button>
                            </Box>
                        ) : (
                            <Accordion allowMultiple>
                                {templates.map((tpl) => (
                                    <AccordionItem key={tpl.id} border="1px solid" borderColor={editingId === tpl.id ? 'orange.300' : 'gray.200'} borderRadius="md" mb={2}>
                                        <h2>
                                            <Flex align="center">
                                                <AccordionButton flex="1" _expanded={{ bg: 'blue.50' }}>
                                                    <Flex flex="1" align="center" gap={3}>
                                                        <Text fontWeight="semibold">{tpl.name}</Text>
                                                        <Badge colorScheme="purple" fontSize="xs">{tpl.items?.length || 0} items</Badge>
                                                        <Text fontSize="xs" color="gray.500">{formatDate(tpl.created_at)}</Text>
                                                    </Flex>
                                                    <AccordionIcon />
                                                </AccordionButton>
                                                <IconButton
                                                    aria-label="Edit template"
                                                    icon={<FiEdit />}
                                                    size="sm"
                                                    colorScheme="blue"
                                                    variant="ghost"
                                                    onClick={() => handleStartEdit(tpl)}
                                                    ml={1}
                                                />
                                                <IconButton
                                                    aria-label="Delete template"
                                                    icon={<FiTrash2 />}
                                                    size="sm"
                                                    colorScheme="red"
                                                    variant="ghost"
                                                    onClick={() => setDeleteId(tpl.id)}
                                                    mr={2}
                                                />
                                            </Flex>
                                        </h2>
                                        <AccordionPanel pb={4}>
                                            {tpl.description && (
                                                <Text fontSize="sm" color="gray.600" mb={3}>{tpl.description}</Text>
                                            )}
                                            <Box overflowX="auto" borderWidth="1px" borderRadius="md">
                                                <Table size="sm" variant="simple">
                                                    <Thead bg="gray.50">
                                                        <Tr>
                                                            <Th>Material</Th>
                                                            <Th>Category</Th>
                                                            <Th isNumeric>Req Qty</Th>
                                                            <Th>Unit</Th>
                                                            <Th>Notes</Th>
                                                        </Tr>
                                                    </Thead>
                                                    <Tbody>
                                                        {tpl.items?.map((item, idx) => (
                                                            <Tr key={item.id ?? idx}>
                                                                <Td>
                                                                    <Text fontSize="sm" fontWeight="medium">
                                                                        {item.component_product
                                                                            ? `${item.component_product.code ? item.component_product.code + ' - ' : ''}${item.component_product.name}`
                                                                            : `Product ID: ${item.component_product_id}`}
                                                                    </Text>
                                                                </Td>
                                                                <Td>
                                                                    <Badge colorScheme="blue" variant="subtle" fontSize="xs">{item.category_str || '—'}</Badge>
                                                                </Td>
                                                                <Td isNumeric>{item.requirement_quantity}</Td>
                                                                <Td>{item.base_unit || '—'}</Td>
                                                                <Td><Text fontSize="xs" color="gray.600">{item.notes || '—'}</Text></Td>
                                                            </Tr>
                                                        ))}
                                                    </Tbody>
                                                </Table>
                                            </Box>
                                        </AccordionPanel>
                                    </AccordionItem>
                                ))}
                            </Accordion>
                        )}
                    </ModalBody>
                    <ModalFooter borderTopWidth="1px" bg="gray.50">
                        <Button onClick={onClose}>Tutup</Button>
                    </ModalFooter>
                </ModalContent>
            </Modal>

            {/* Delete Confirmation */}
            <AlertDialog isOpen={deleteId !== null} leastDestructiveRef={cancelRef as any} onClose={() => setDeleteId(null)}>
                <AlertDialogOverlay>
                    <AlertDialogContent>
                        <AlertDialogHeader>Hapus Template</AlertDialogHeader>
                        <AlertDialogBody>Apakah Anda yakin ingin menghapus template ini? Aksi ini tidak dapat dibatalkan.</AlertDialogBody>
                        <AlertDialogFooter>
                            <Button ref={cancelRef as any} onClick={() => setDeleteId(null)}>Batal</Button>
                            <Button colorScheme="red" onClick={handleDelete} ml={3}>Hapus</Button>
                        </AlertDialogFooter>
                    </AlertDialogContent>
                </AlertDialogOverlay>
            </AlertDialog>
        </>
    );
};

export default ComponentTemplateManagerModal;
