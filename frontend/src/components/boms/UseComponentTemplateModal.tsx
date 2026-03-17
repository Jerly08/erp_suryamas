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
    Text,
    Box,
    Badge,
    Flex,
    Spinner,
    Radio,
    RadioGroup,
    VStack,
    Table,
    Thead,
    Tbody,
    Tr,
    Th,
    Td,
    Divider,
    useColorModeValue,
} from '@chakra-ui/react';
import { componentTemplateService } from '@/services/bomService';
import { ComponentTemplate } from '@/types/bom';

interface UseComponentTemplateModalProps {
    isOpen: boolean;
    onClose: () => void;
    onLoad: (items: {
        component_product_id: number;
        category_str: string;
        requirement_quantity: number;
        base_unit: string;
        notes: string;
    }[]) => void;
}

const UseComponentTemplateModal: React.FC<UseComponentTemplateModalProps> = ({ isOpen, onClose, onLoad }) => {
    const [templates, setTemplates] = useState<ComponentTemplate[]>([]);
    const [loading, setLoading] = useState(false);
    const [selectedId, setSelectedId] = useState<string>('');
    const [previewTemplate, setPreviewTemplate] = useState<ComponentTemplate | null>(null);
    const [loadingPreview, setLoadingPreview] = useState(false);
    const modalBg = useColorModeValue('white', 'var(--bg-secondary)');
    const cardBg = useColorModeValue('white', 'var(--bg-secondary)');
    const selectedBg = useColorModeValue('blue.50', 'whiteAlpha.100');
    const borderColor = useColorModeValue('gray.200', 'var(--border-color)');
    const selectedBorderColor = useColorModeValue('blue.400', 'blue.300');
    const hoverBorderColor = useColorModeValue('blue.300', 'blue.200');
    const textColor = useColorModeValue('gray.700', 'var(--text-primary)');
    const mutedTextColor = useColorModeValue('gray.500', 'var(--text-secondary)');
    const tableHeadBg = useColorModeValue('gray.50', 'var(--bg-tertiary)');
    const footerBg = useColorModeValue('gray.50', 'whiteAlpha.100');

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

    useEffect(() => {
        if (isOpen) {
            fetchTemplates();
            setSelectedId('');
            setPreviewTemplate(null);
        }
    }, [isOpen]);

    const handleSelectTemplate = async (idStr: string) => {
        setSelectedId(idStr);
        const id = parseInt(idStr);
        setLoadingPreview(true);
        try {
            const response = await componentTemplateService.getTemplate(id);
            if (response.data) {
                setPreviewTemplate(response.data);
            }
        } catch (error) {
            console.error('Failed to fetch template detail', error);
        } finally {
            setLoadingPreview(false);
        }
    };

    const handleLoad = () => {
        if (!previewTemplate || !previewTemplate.items) return;
        const items = previewTemplate.items.map((item) => ({
            component_product_id: item.component_product_id,
            category_str: item.category_str,
            requirement_quantity: item.requirement_quantity,
            base_unit: item.base_unit,
            notes: item.notes || '',
        }));
        onLoad(items);
        onClose();
    };

    return (
        <Modal isOpen={isOpen} onClose={onClose} size="3xl" scrollBehavior="inside">
            <ModalOverlay />
            <ModalContent maxH="85vh" bg={modalBg} border="1px solid" borderColor={borderColor}>
                <ModalHeader borderBottomWidth="1px" borderColor={borderColor} color={textColor}>Pilih Component Template</ModalHeader>
                <ModalCloseButton />
                <ModalBody pb={4}>
                    {loading ? (
                        <Flex justify="center" py={10}><Spinner size="lg" color="blue.500" /></Flex>
                    ) : templates.length === 0 ? (
                        <Box textAlign="center" py={10}>
                            <Text color={mutedTextColor}>
                                Belum ada template tersimpan. Buat BOM terlebih dahulu dan centang &quot;Save components as template&quot;.
                            </Text>
                        </Box>
                    ) : (
                        <VStack spacing={4} align="stretch">
                            <RadioGroup value={selectedId} onChange={handleSelectTemplate}>
                                <VStack align="stretch" spacing={2}>
                                    {templates.map((tpl) => (
                                        <Box
                                            key={tpl.id}
                                            p={3}
                                            borderWidth="1px"
                                            borderRadius="md"
                                            cursor="pointer"
                                            _hover={{ borderColor: hoverBorderColor, bg: selectedBg }}
                                            borderColor={selectedId === tpl.id.toString() ? selectedBorderColor : borderColor}
                                            bg={selectedId === tpl.id.toString() ? selectedBg : cardBg}
                                            onClick={() => handleSelectTemplate(tpl.id.toString())}
                                        >
                                            <Flex align="center" gap={3}>
                                                <Radio value={tpl.id.toString()} />
                                                <Box flex="1">
                                                    <Text fontWeight="semibold" color={textColor}>{tpl.name}</Text>
                                                    {tpl.description && (
                                                        <Text fontSize="xs" color={mutedTextColor}>{tpl.description}</Text>
                                                    )}
                                                </Box>
                                                <Badge colorScheme="purple" fontSize="xs">{tpl.items?.length || 0} items</Badge>
                                            </Flex>
                                        </Box>
                                    ))}
                                </VStack>
                            </RadioGroup>

                            {/* Preview selected template */}
                            {selectedId && (
                                <>
                                    <Divider />
                                    <Box>
                                        <Text fontWeight="semibold" mb={2} color={textColor}>Preview komponen:</Text>
                                        {loadingPreview ? (
                                            <Flex justify="center" py={4}><Spinner size="sm" /></Flex>
                                        ) : previewTemplate ? (
                                            <Box overflowX="auto" borderWidth="1px" borderRadius="md" maxH="200px" overflowY="auto" borderColor={borderColor}>
                                                <Table size="sm" variant="simple">
                                                    <Thead bg={tableHeadBg} position="sticky" top={0} zIndex={1}>
                                                        <Tr>
                                                            <Th>Material</Th>
                                                            <Th>Category</Th>
                                                            <Th isNumeric>Req Qty</Th>
                                                            <Th>Unit</Th>
                                                        </Tr>
                                                    </Thead>
                                                    <Tbody>
                                                        {previewTemplate.items?.map((item, idx) => (
                                                            <Tr key={item.id ?? idx}>
                                                                <Td>
                                                                    <Text fontSize="sm" color={textColor}>
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
                                                            </Tr>
                                                        ))}
                                                    </Tbody>
                                                </Table>
                                            </Box>
                                        ) : null}
                                    </Box>
                                </>
                            )}
                        </VStack>
                    )}
                </ModalBody>
                <ModalFooter borderTopWidth="1px" borderColor={borderColor} bg={footerBg}>
                    <Button variant="ghost" mr={3} onClick={onClose}>Batal</Button>
                    <Button
                        colorScheme="blue"
                        onClick={handleLoad}
                        isDisabled={!selectedId || !previewTemplate}
                    >
                        Load Template ({previewTemplate?.items?.length || 0} items)
                    </Button>
                </ModalFooter>
            </ModalContent>
        </Modal>
    );
};

export default UseComponentTemplateModal;
