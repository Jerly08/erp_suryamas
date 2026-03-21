"use client";

import React from "react";
import SimpleLayout from "@/components/layout/SimpleLayout";
import ProtectedModule from "@/components/common/ProtectedModule";
import {
  Tabs,
  TabList,
  TabPanels,
  Tab,
  TabPanel,
  Box,
  Heading,
  useColorModeValue,
} from "@chakra-ui/react";
import { useTranslation } from "@/hooks/useTranslation";
import { useModulePermissions } from "@/hooks/usePermissions";
import GeneralJournalView from "@/components/journal/GeneralJournalView";
import GeneralLedgerView from "@/components/journal/GeneralLedgerView";
import BalanceSheetView from "@/components/journal/BalanceSheetView";

import CompanyTypeGuard from "@/components/common/CompanyTypeGuard";

export default function JournalPage() {
  const { t } = useTranslation();
  const { canApprove } = useModulePermissions("journal");
  const bg = useColorModeValue("white", "gray.800");
  const headingColor = useColorModeValue("gray.800", "gray.100");
  const tabListBorderColor = useColorModeValue("gray.200", "whiteAlpha.200");
  const tabColor = useColorModeValue("gray.600", "gray.300");
  const tabHoverBg = useColorModeValue("gray.50", "whiteAlpha.100");
  const activeTabBg = useColorModeValue("blue.50", "rgba(66, 153, 225, 0.16)");
  const activeTabColor = useColorModeValue("blue.700", "blue.200");
  return (
    <ProtectedModule module="journal">
      <CompanyTypeGuard>
        <SimpleLayout>
          <Box bg={bg} borderRadius="lg" p={4} shadow="sm">
            <Heading size="lg" mb={6} color={headingColor}>
              {t("navigation.journal") || "Jurnal"}
            </Heading>
            <Tabs variant="enclosed" colorScheme="blue" isLazy>
              <TabList borderColor={tabListBorderColor}>
                <Tab
                  color={tabColor}
                  _hover={{ bg: tabHoverBg }}
                  _selected={{
                    bg: activeTabBg,
                    color: activeTabColor,
                    borderColor: tabListBorderColor,
                  }}
                >
                  {t("journal.generalJournal") || "Jurnal Umum"}
                </Tab>
                {canApprove && (
                  <Tab
                    color={tabColor}
                    _hover={{ bg: tabHoverBg }}
                    _selected={{
                      bg: activeTabBg,
                      color: activeTabColor,
                      borderColor: tabListBorderColor,
                    }}
                  >
                    {t("journal.generalLedger") || "Buku Besar"}
                  </Tab>
                )}
                {canApprove && (
                  <Tab
                    color={tabColor}
                    _hover={{ bg: tabHoverBg }}
                    _selected={{
                      bg: activeTabBg,
                      color: activeTabColor,
                      borderColor: tabListBorderColor,
                    }}
                  >
                    {t("journal.balanceSheet") || "Neraca"}
                  </Tab>
                )}
              </TabList>

              <TabPanels>
                <TabPanel px={0}>
                  <GeneralJournalView />
                </TabPanel>
                {canApprove && (
                  <TabPanel px={0}>
                    <GeneralLedgerView />
                  </TabPanel>
                )}
                {canApprove && (
                  <TabPanel px={0}>
                    <BalanceSheetView />
                  </TabPanel>
                )}
              </TabPanels>
            </Tabs>
          </Box>
        </SimpleLayout>
      </CompanyTypeGuard>
    </ProtectedModule>
  );
}
