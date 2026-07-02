import { useState, useEffect, useRef } from "react";
import api from "@/services/api";
import { API_ENDPOINTS } from "@/config/api";
import {
  isFinanceManagerRole,
  isFinanceStaffRole,
  normalizeRole,
} from "@/utils/roles";

// Define the structure of the analytics data
interface DashboardAnalytics {
  totalSales: number;
  totalPurchases: number;
  accountsReceivable: number;
  accountsPayable: number;

  // Growth percentages
  salesGrowth: number;
  purchasesGrowth: number;
  receivablesGrowth: number;
  payablesGrowth: number;

  monthlySales: { month: string; value: number }[];
  monthlyPurchases: { month: string; value: number }[];
  cashFlow: {
    month: string;
    inflow: number;
    outflow: number;
    balance: number;
  }[];
  topAccounts: { name: string; balance: number; type: string }[];
  recentTransactions: any[];
}

// Cache for analytics data with timestamp
interface CacheEntry {
  data: DashboardAnalytics;
  timestamp: number;
}

const analyticsCache: { current: CacheEntry | null } = { current: null };
const CACHE_DURATION = 30000; // 30 seconds cache

export const useDashboardAnalytics = (
  user: any,
  enabled = true,
) => {
  const [analytics, setAnalytics] = useState<DashboardAnalytics | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const fetchingRef = useRef(false);

  useEffect(() => {
    if (!enabled || !user) {
      setLoading(false);
      return;
    }

    // Check if user has required role
    const userRoleNormalized = normalizeRole(user.role);
    if (
      userRoleNormalized !== "admin" &&
      userRoleNormalized !== "director" &&
      !isFinanceManagerRole(userRoleNormalized) &&
      !isFinanceStaffRole(userRoleNormalized)
    ) {
      setLoading(false);
      return;
    }

    const fetchAnalytics = async () => {
      // Prevent duplicate API calls
      if (fetchingRef.current) {
        console.log(
          "⚠️ Dashboard analytics fetch already in progress, skipping duplicate call",
        );
        return;
      }

      // Check cache first
      if (analyticsCache.current) {
        const now = Date.now();
        const cacheAge = now - analyticsCache.current.timestamp;
        if (cacheAge < CACHE_DURATION) {
          console.log(
            "📦 Using cached dashboard analytics data (age:",
            Math.round(cacheAge / 1000),
            "seconds)",
          );
          setAnalytics(analyticsCache.current.data);
          setError(null);
          setLoading(false);
          return;
        }
      }

      try {
        fetchingRef.current = true;
        console.log("🔍 Dashboard Debug Info:");
        console.log("User:", user);
        console.log("User role:", user.role);
        console.log(
          "User role normalized:",
          user.role?.toString().toLowerCase(),
        );

        console.log(
          "🌐 Making API request to",
          API_ENDPOINTS.DASHBOARD_ANALYTICS,
        );
        const response = await api.get(API_ENDPOINTS.DASHBOARD_ANALYTICS);

        console.log(
          "✅ Dashboard analytics response received:",
          Object.keys(response.data),
        );

        // Update cache
        analyticsCache.current = {
          data: response.data,
          timestamp: Date.now(),
        };

        setAnalytics(response.data);
        setError(null);
      } catch (err: any) {
        console.error("❌ Failed to fetch dashboard analytics:", err);

        // Handle authentication errors
        if (err.response?.status === 401) {
          console.error(
            "🚫 Authentication failed - token might be expired or invalid",
          );
          setError("Session expired. Please login again.");
          return;
        }

        // Handle authorization errors
        if (err.response?.status === 403) {
          console.error("🚫 User not authorized to view dashboard analytics");
          setError(
            `You are not authorized to view dashboard analytics. Current role: ${user.role}`,
          );
        } else {
          const errorMessage =
            err.response?.data?.error ||
            err.message ||
            "Failed to load dashboard data";
          setError(errorMessage);
        }
      } finally {
        setLoading(false);
        fetchingRef.current = false;
      }
    };

    fetchAnalytics();
  }, [enabled, user]);

  return { analytics, loading, error };
};
