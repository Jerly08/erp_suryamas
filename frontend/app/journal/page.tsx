'use client';

import React from 'react';
import SimpleLayout from '@/components/layout/SimpleLayout';
import { useModulePermissions } from '@/hooks/usePermissions';
import ProtectedModule from '@/components/common/ProtectedModule';
import {
    Tabs,
    TabList,
    TabPanels,
    Tab,
    TabPanel,
    Box,
    Heading,
    useColorModeValue,
} from '@chakra-ui/react';
import { useTranslation } from '@/hooks/useTranslation';
import GeneralJournalView from '@/components/journal/GeneralJournalView';
import GeneralLedgerView from '@/components/journal/GeneralLedgerView';
import BalanceSheetView from '@/components/journal/BalanceSheetView';

export default function JournalPage() {
    const { t } = useTranslation();
    const bg = useColorModeValue('white', 'gray.800');

    // Permission-based access control
    const { canView, canCreate, canEdit, canDelete, canExport } = useModulePermissions('journal');

    return (
        <ProtectedModule module="journal">
            <SimpleLayout>
                <Box bg={bg} borderRadius="lg" p={4} shadow="sm">
                    <Heading size="lg" mb={6}>{t('navigation.journal') || 'Jurnal'}</Heading>
                    <Tabs variant="enclosed" colorScheme="blue" isLazy>
                        <TabList>
                            <Tab>{t('journal.generalJournal') || 'Jurnal Umum'}</Tab>
                            <Tab>{t('journal.generalLedger') || 'Buku Besar'}</Tab>
                            <Tab>{t('journal.balanceSheet') || 'Neraca'}</Tab>
                        </TabList>

                        <TabPanels>
                            <TabPanel px={0}>
                                <GeneralJournalView />
                            </TabPanel>
                            <TabPanel px={0}>
                                <GeneralLedgerView />
                            </TabPanel>
                            <TabPanel px={0}>
                                <BalanceSheetView />
                            </TabPanel>
                        </TabPanels>
                    </Tabs>
                </Box>
            </SimpleLayout>
        </ProtectedModule>
    );
}

