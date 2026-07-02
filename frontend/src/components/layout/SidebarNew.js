"use client";

import React from "react";
import { usePathname, useRouter } from "next/navigation";
import {
  Box,
  CloseButton,
  Flex,
  Image,
  Icon,
  useColorModeValue,
  Text,
  Drawer,
  DrawerContent,
  DrawerOverlay,
} from "@chakra-ui/react";
import {
  FiTrendingUp,
  FiCompass,
  FiStar,
  FiClock,
  FiSettings,
  FiShoppingCart,
  FiUsers,
  FiDollarSign,
  FiBarChart,
  FiFileText,
  FiHome,
  FiLayers,
  FiUser,
  FiBox,
  FiBriefcase,
  FiClipboard,
} from "react-icons/fi";
import { useAuth } from "@/contexts/AuthContext";
import { normalizeRole } from "@/utils/roles";
import { useTranslation } from "@/hooks/useTranslation";
import { useTheme } from "@/contexts/SimpleThemeContext";
import { usePermissions } from "@/hooks/usePermissions";
import { AVERY_LOGO_ALT, AVERY_LOGO_PATH, POWERED_BY_TEXT } from "@/config/branding";

// Menu structure - will be translated dynamically
// companyType: 'SERVICE' | 'TRADING' | 'MANUFACTURING'
const getMenuGroups = (t, companyType) => {
  // Helper: get company-type label or fallback to generic translation
  const ctLabel = (key, fallbackKey) => {
    if (companyType) {
      const translated = t(`companyType.${companyType}.${key}`);
      if (!translated.startsWith("companyType.")) return translated;
    }
    return t(fallbackKey);
  };

  return [
    {
      title: t("navigation.dashboard"),
      items: [
        {
          name: t("navigation.dashboard"),
          icon: FiHome,
          href: "/dashboard",
          module: null,
          permission: null,
          roles: [
            "ADMIN",
            "FINANCE",
            "FINANCE_STAFF",
            "ACCOUNTING",
            "HR",
            "INVENTORY_MANAGER",
            "DIRECTOR",
            "EMPLOYEE",
          ],
        },
      ],
    },
    {
      title: t("navigation.sidebar.masterData"),
      items: [
        {
          name: t("navigation.accounts"),
          icon: FiFileText,
          href: "/accounts",
          module: "accounts",
          permission: "view",
          roles: ["ADMIN", "FINANCE"],
        },
        {
          name: ctLabel("products", "Items & Benchmarks"),
          icon: FiLayers,
          href: "/products",
          module: "products",
          permission: "view",
          roles: ["ADMIN", "INVENTORY_MANAGER", "EMPLOYEE", "DIRECTOR"],
        },
        {
          name: "Master BOMs",
          icon: FiBox,
          href: "/boms",
          module: "boms",
          permission: "view",
          roles: [
            "ADMIN",
            "FINANCE",
            "INVENTORY_MANAGER",
            "EMPLOYEE",
            "DIRECTOR",
          ],
        },
        {
          name: t("navigation.contacts"),
          icon: FiUsers,
          href: "/contacts",
          module: "contacts",
          permission: "view",
          roles: [
            "ADMIN",
            "FINANCE",
            "INVENTORY_MANAGER",
            "EMPLOYEE",
            "DIRECTOR",
          ],
        },
        {
          name: t("navigation.assets"),
          icon: FiStar,
          FiClock,
          href: "/assets",
          module: "assets",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
        {
          name: "Prepaid Expenses",
          icon: FiClock,
          href: "/prepaid-expenses",
          module: "prepaid_expenses",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
        {
          name: "Projects",
          icon: FiBriefcase,
          href: "/projects",
          module: "projects",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
      ],
    },
    {
      title: t("navigation.sidebar.financial"),
      items: [
        {
          name: "Purchase Requests",
          icon: FiClipboard,
          href: "/purchase-requests",
          module: "purchase_requests",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "EMPLOYEE", "DIRECTOR"],
        },
        {
          name: ctLabel("sales", "navigation.sales"),
          icon: FiDollarSign,
          href: "/sales",
          module: "sales",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR", "EMPLOYEE"],
        },
        {
          name: ctLabel("purchases", "navigation.purchases"),
          icon: FiShoppingCart,
          href: "/purchases",
          module: "purchases",
          permission: "view",
          roles: [
            "ADMIN",
            "FINANCE",
            "INVENTORY_MANAGER",
            "EMPLOYEE",
            "DIRECTOR",
          ],
        },
        {
          name: t("navigation.payments"),
          icon: FiTrendingUp,
          href: "/payments",
          module: "payments",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
        {
          name: t("navigation.cashBank"),
          icon: FiCompass,
          href: "/cash-bank",
          module: "cash_bank",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
        {
          name: t("navigation.journal"),
          icon: FiFileText,
          href: "/journal",
          module: "journal",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
        {
          name: t("navigation.payroll"),
          icon: FiUsers,
          href: "/payroll",
          module: "payroll",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "FINANCE_STAFF", "ACCOUNTING", "HR"],
        },
      ],
    },
    {
      title: t("navigation.reports"),
      items: [
        {
          name: t("navigation.reports"),
          icon: FiBarChart,
          href: "/reports",
          module: "reports",
          permission: "view",
          roles: ["ADMIN", "FINANCE", "DIRECTOR"],
        },
      ],
    },
    {
      title: t("navigation.sidebar.system"),
      items: [
        {
          name: t("navigation.users"),
          icon: FiUser,
          href: "/users",
          module: "users",
          permission: "view",
          roles: ["ADMIN"],
        },
        {
          name: t("navigation.settings"),
          icon: FiSettings,
          href: "/settings",
          module: "settings",
          permission: "view",
          roles: ["ADMIN"],
        },
      ],
    },
  ];
};

export default function Sidebar({
  isOpen,
  onClose,
  display,
  width,
  collapsed,
  onToggleCollapse,
  variant,
  ...rest
}) {
  const { user } = useAuth();
  const pathname = usePathname();
  const router = useRouter();
  const { t } = useTranslation();
  const { theme } = useTheme();
  const { canView, canMenu, loading: permissionLoading } = usePermissions();

  // Get company type from user context
  const companyType = user?.company_type || "";

  // Get translated menu groups with company-type-specific labels
  const MenuGroups = getMenuGroups(t, companyType);

  // Filter menu groups based on managed permissions.
  // Role lists are only used for non-module entries like Dashboard.
  const userRoleNormalized = user ? normalizeRole(user.role) : "";
  const filteredGroups = MenuGroups.map((group) => ({
    ...group,
    items: group.items.filter((item) => {
      if (!user) return false;

      if (item.module && item.permission) {
        return canView(item.module) && canMenu(item.module);
      }

      return item.roles.some(
        (role) => normalizeRole(role) === userRoleNormalized,
      );
    }),
  })).filter((group) => group.items.length > 0);

  const SidebarContent = ({ onClose, ...rest }) => {
    // Theme-aware colors
    const sidebarBg = useColorModeValue("white", "var(--bg-primary)");
    const sidebarBorder = useColorModeValue("gray.200", "var(--border-color)");
    const sidebarShadow = useColorModeValue("sm", "var(--shadow)");
    const closeButtonColor = useColorModeValue(
      "gray.600",
      "var(--text-secondary)",
    );
    const closeButtonHoverBg = useColorModeValue(
      "gray.100",
      "var(--bg-tertiary)",
    );
    const groupLabelColor = useColorModeValue(
      "gray.500",
      "var(--text-secondary)",
    );

    return (
      <Box
        transition="all 0.3s ease"
        bg={sidebarBg}
        borderRight="1px"
        borderRightColor={sidebarBorder}
        w={{ base: "full", lg: width || 60 }}
        maxW={{ base: "min(86vw, 320px)", lg: "none" }}
        pos="fixed"
        h="full"
        overflowY="auto"
        zIndex={1000}
        className="sidebar"
        boxShadow={sidebarShadow}
        display="flex"
        flexDirection="column"
        {...rest}
      >
        <Flex
          h={{ base: "16", md: "20" }}
          alignItems="center"
          mx={{ base: "4", md: "8" }}
          justifyContent="space-between"
        >
          <Image
            src={AVERY_LOGO_PATH}
            alt={AVERY_LOGO_ALT}
            h={{ base: "34px", md: "42px" }}
            maxW="150px"
            objectFit="contain"
          />
          <CloseButton
            display={{ base: "flex", lg: "none" }}
            onClick={onClose}
            color={closeButtonColor}
            _hover={{ bg: closeButtonHoverBg }}
            aria-label={t("navigation.navbar.closeMenu")}
          />
        </Flex>

        <Box flex="1">
          {filteredGroups.map((group) => (
            <Box key={group.title} mb={6}>
              <Text
                fontSize="xs"
                fontWeight="semibold"
                color={groupLabelColor}
                textTransform="uppercase"
                mx="4"
                mb="3"
                letterSpacing="wider"
                opacity={0.8}
              >
                {group.title}
              </Text>
              {group.items.map((link) => (
                <NavItem
                  key={link.name}
                  icon={link.icon}
                  href={link.href}
                  isActive={pathname === link.href}
                  onNavigate={onClose}
                >
                  {link.name}
                </NavItem>
              ))}
            </Box>
          ))}
        </Box>

        <Box px="6" py="4" borderTop="1px solid" borderTopColor={sidebarBorder}>
          <Text fontSize="10px" color={groupLabelColor} lineHeight="short" opacity={0.75}>
            {POWERED_BY_TEXT}
          </Text>
        </Box>
      </Box>
    );
  };

  const NavItem = ({ icon, children, href, isActive, onNavigate, ...rest }) => {
    const handleClick = (e) => {
      e.preventDefault();
      console.log("NavItem clicked:", href);
      router.push(href);
      if (typeof onNavigate === "function") {
        onNavigate();
      }
    };

    // Theme-aware colors for nav items
    const activeBg = "var(--accent-color)";
    const activeColor = "white";
    const inactiveColor = useColorModeValue(
      "gray.700",
      "var(--text-secondary)",
    );
    const hoverBg = useColorModeValue("gray.100", "var(--bg-tertiary)");
    const hoverColor = useColorModeValue("gray.900", "var(--text-primary)");
    const iconColor = useColorModeValue("gray.500", "var(--text-secondary)");
    const iconActiveColor = "white";
    const iconHoverColor = useColorModeValue("gray.700", "var(--text-primary)");

    return (
      <Flex
        onClick={handleClick}
        align="center"
        p="3"
        mx="4"
        minH="44px"
        borderRadius="lg"
        role="group"
        cursor="pointer"
        className="sidebar-item"
        bg={isActive ? activeBg : "transparent"}
        color={isActive ? activeColor : inactiveColor}
        borderLeft="3px solid"
        borderLeftColor={isActive ? "var(--accent-color)" : "transparent"}
        _hover={{
          bg: isActive ? "var(--accent-color)" : hoverBg,
          color: isActive ? "white" : hoverColor,
          transform: "translateX(4px)",
        }}
        transition="all 0.2s ease"
        fontWeight="medium"
        fontSize="sm"
        pointerEvents="auto"
        zIndex={10}
        wordBreak="break-word"
        {...rest}
      >
        {icon && (
          <Icon
            mr="3"
            fontSize="18"
            color={isActive ? iconActiveColor : iconColor}
            _groupHover={{
              color: isActive ? "white" : iconHoverColor,
            }}
            as={icon}
            transition="all 0.2s ease"
          />
        )}
        {children}
      </Flex>
    );
  };

  if (variant === "drawer") {
    return (
      <Drawer
        autoFocus={false}
        isOpen={isOpen}
        placement="left"
        onClose={onClose}
        returnFocusOnClose={false}
        onOverlayClick={onClose}
        size="xs"
      >
        <DrawerOverlay />
        <DrawerContent maxW={{ base: "min(86vw, 320px)", sm: "320px" }}>
          <SidebarContent onClose={onClose} />
        </DrawerContent>
      </Drawer>
    );
  }

  return <SidebarContent onClose={() => onClose} display={display} />;
}
