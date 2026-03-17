import React, { useState, useEffect, useCallback } from 'react';
import { 
  Card, 
  CardContent, 
  CardHeader, 
  CardTitle, 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue,
  Badge,
  Alert,
  AlertDescription,
  Button,
  Tabs,
  TabsContent,
  TabsList,
  TabsTrigger,
  Separator,
  Tooltip,
  TooltipContent,
  TooltipProvider,
  TooltipTrigger
} from '@/components/ui';
import { 
  CheckCircle, 
  AlertTriangle, 
  XCircle, 
  Info, 
  RefreshCw,
  Eye,
  Settings,
  BookOpen
} from 'lucide-react';

interface Account {
  id: number;
  code: string;
  name: string;
  type: string;
  category: string;
  is_active: boolean;
}

interface AccountStatus {
  is_configured: boolean;
  account?: Account;
  warnings?: string[];
  recommendations?: string[];
}

interface TaxAccountStatus {
  is_fully_configured: boolean;
  configuration_id?: number;
  last_updated?: string;
  updated_by?: {
    id: number;
    name: string;
    username: string;
  };
  sales_receivable: AccountStatus;
  sales_cash: AccountStatus;
  sales_bank: AccountStatus;
  sales_revenue: AccountStatus;
  sales_output_vat: AccountStatus;
  purchase_payable: AccountStatus;
  purchase_cash: AccountStatus;
  purchase_bank: AccountStatus;
  purchase_input_vat: AccountStatus;
  purchase_expense: AccountStatus;
  missing_accounts: string[];
  system_warnings: string[];
  health_score: number;
}

interface ValidationResult {
  is_valid: boolean;
  warnings: string[];
  recommendations: string[];
  account: Account;
}

interface EnhancedTaxAccountSettingsProps {
  onSettingsChange?: (settings: any) => void;
}

const EnhancedTaxAccountSettings: React.FC<EnhancedTaxAccountSettingsProps> = ({ 
  onSettingsChange 
}) => {
  const [status, setStatus] = useState<TaxAccountStatus | null>(null);
  const [accounts, setAccounts] = useState<Account[]>([]);
  const [loading, setLoading] = useState(true);
  const [refreshing, setRefreshing] = useState(false);
  const [selectedAccounts, setSelectedAccounts] = useState<Record<string, number>>({});
  const [validationResults, setValidationResults] = useState<Record<string, ValidationResult>>({});

  const fetchStatus = useCallback(async () => {
    try {
      setRefreshing(true);
      const response = await fetch('/api/v1/tax-accounts/status');
      const data = await response.json();
      
      if (data.success) {
        setStatus(data.data);
        
        // Extract currently selected accounts
        const currentSelections: Record<string, number> = {};
        if (data.data.sales_receivable.account) currentSelections.sales_receivable = data.data.sales_receivable.account.id;
        if (data.data.sales_cash.account) currentSelections.sales_cash = data.data.sales_cash.account.id;
        if (data.data.sales_bank.account) currentSelections.sales_bank = data.data.sales_bank.account.id;
        if (data.data.sales_revenue.account) currentSelections.sales_revenue = data.data.sales_revenue.account.id;
        if (data.data.sales_output_vat.account) currentSelections.sales_output_vat = data.data.sales_output_vat.account.id;
        if (data.data.purchase_payable.account) currentSelections.purchase_payable = data.data.purchase_payable.account.id;
        if (data.data.purchase_cash.account) currentSelections.purchase_cash = data.data.purchase_cash.account.id;
        if (data.data.purchase_bank.account) currentSelections.purchase_bank = data.data.purchase_bank.account.id;
        if (data.data.purchase_input_vat.account) currentSelections.purchase_input_vat = data.data.purchase_input_vat.account.id;
        if (data.data.purchase_expense.account) currentSelections.purchase_expense = data.data.purchase_expense.account.id;
        
        setSelectedAccounts(currentSelections);
      }
    } catch (error) {
      console.error('Failed to fetch status:', error);
    } finally {
      setRefreshing(false);
    }
  }, []);

  const fetchAccounts = useCallback(async () => {
    try {
      const response = await fetch('/api/v1/accounts?per_page=1000');
      const data = await response.json();
      
      if (data.success) {
        setAccounts(data.data || []);
      }
    } catch (error) {
      console.error('Failed to fetch accounts:', error);
    }
  }, []);

  const validateAccountSelection = async (accountId: number, role: string) => {
    try {
      const response = await fetch(
        `/api/v1/tax-accounts/validate?account_id=${accountId}&role=${role}`
      );
      const data = await response.json();
      
      if (data.success) {
        setValidationResults(prev => ({
          ...prev,
          [role]: data.data
        }));
      }
    } catch (error) {
      console.error('Validation failed:', error);
    }
  };

  useEffect(() => {
    const initialize = async () => {
      setLoading(true);
      await Promise.all([fetchStatus(), fetchAccounts()]);
      setLoading(false);
    };

    initialize();
  }, [fetchStatus, fetchAccounts]);

  const handleAccountChange = (role: string, accountId: string) => {
    const id = parseInt(accountId);
    setSelectedAccounts(prev => ({
      ...prev,
      [role]: id
    }));
    
    // Validate the selection
    validateAccountSelection(id, role);
    
    // Notify parent component
    if (onSettingsChange) {
      onSettingsChange({
        ...selectedAccounts,
        [role]: id
      });
    }
  };

  const getStatusIcon = (accountStatus: AccountStatus) => {
    if (!accountStatus.is_configured) {
      return <XCircle className="h-4 w-4 text-red-500" />;
    }
    
    if (accountStatus.warnings && accountStatus.warnings.length > 0) {
      return <AlertTriangle className="h-4 w-4 text-yellow-500" />;
    }
    
    return <CheckCircle className="h-4 w-4 text-green-500" />;
  };

  const getHealthScoreColor = (score: number) => {
    if (score >= 80) return 'text-green-600';
    if (score >= 60) return 'text-yellow-600';
    return 'text-red-600';
  };

  const getHealthScoreBadgeVariant = (score: number) => {
    if (score >= 80) return 'default';
    if (score >= 60) return 'secondary';
    return 'destructive';
  };

  const AccountSelector = ({ 
    role, 
    label, 
    description,
    accountStatus,
    expectedType 
  }: {
    role: string;
    label: string;
    description: string;
    accountStatus: AccountStatus;
    expectedType: string;
  }) => {
    const currentAccount = accountStatus.account;
    const validation = validationResults[role];
    const filteredAccounts = accounts.filter(account => account.type === expectedType && account.is_active);
    
    return (
      <div className="space-y-2">
        <div className="flex items-center justify-between">
          <div className="flex items-center space-x-2">
            {getStatusIcon(accountStatus)}
            <label className="text-sm font-medium">{label}</label>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <Info className="h-3 w-3 text-gray-400" />
                </TooltipTrigger>
                <TooltipContent>
                  <p className="text-xs max-w-xs">{description}</p>
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </div>
          
          {currentAccount && (
            <Badge variant="outline" className="text-xs">
              [{currentAccount.code}] {currentAccount.name}
            </Badge>
          )}
        </div>
        
        <Select
          value={selectedAccounts[role]?.toString() || ''}
          onValueChange={(value) => handleAccountChange(role, value)}
        >
          <SelectTrigger className={`w-full ${
            validation && !validation.is_valid ? 'border-red-300' : 
            accountStatus.warnings && accountStatus.warnings.length > 0 ? 'border-yellow-300' : 
            accountStatus.is_configured ? 'border-green-300' : ''
          }`}>
            <SelectValue placeholder={`Select ${label.toLowerCase()}`} />
          </SelectTrigger>
          <SelectContent>
            {filteredAccounts.map((account) => (
              <SelectItem key={account.id} value={account.id.toString()}>
                <div className="flex items-center justify-between w-full">
                  <span>[{account.code}] {account.name}</span>
                  <Badge variant="secondary" className="text-xs ml-2">
                    {account.type}
                  </Badge>
                </div>
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
        
        {/* Validation Messages */}
        {validation && validation.warnings.length > 0 && (
          <Alert className="mt-2">
            <AlertTriangle className="h-4 w-4" />
            <AlertDescription className="text-xs">
              {validation.warnings.map((warning, index) => (
                <div key={index}>{warning}</div>
              ))}
            </AlertDescription>
          </Alert>
        )}
        
        {validation && validation.recommendations.length > 0 && (
          <Alert className="mt-2">
            <Info className="h-4 w-4" />
            <AlertDescription className="text-xs">
              {validation.recommendations.map((recommendation, index) => (
                <div key={index}>{recommendation}</div>
              ))}
            </AlertDescription>
          </Alert>
        )}
      </div>
    );
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center p-8">
        <RefreshCw className="h-6 w-6 animate-spin" />
        <span className="ml-2">Loading configuration...</span>
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Status Header */}
      {status && (
        <Card>
          <CardHeader className="pb-3">
            <div className="flex items-center justify-between">
              <CardTitle className="flex items-center space-x-2">
                <Settings className="h-5 w-5" />
                <span>Tax Account Configuration Status</span>
              </CardTitle>
              <Button
                variant="outline"
                size="sm"
                onClick={fetchStatus}
                disabled={refreshing}
              >
                <RefreshCw className={`h-4 w-4 mr-2 ${refreshing ? 'animate-spin' : ''}`} />
                Refresh
              </Button>
            </div>
          </CardHeader>
          <CardContent>
            <div className="grid grid-cols-1 md:grid-cols-3 gap-4 mb-4">
              <div className="text-center">
                <div className={`text-2xl font-bold ${getHealthScoreColor(status.health_score)}`}>
                  {status.health_score}%
                </div>
                <div className="text-sm text-gray-500">Health Score</div>
              </div>
              <div className="text-center">
                <Badge variant={status.is_fully_configured ? 'default' : 'destructive'}>
                  {status.is_fully_configured ? 'Fully Configured' : 'Incomplete'}
                </Badge>
                <div className="text-sm text-gray-500 mt-1">Configuration Status</div>
              </div>
              <div className="text-center">
                {status.last_updated && (
                  <div>
                    <div className="text-sm font-medium">{status.last_updated}</div>
                    <div className="text-xs text-gray-500">
                      by {status.updated_by?.name || 'System'}
                    </div>
                  </div>
                )}
              </div>
            </div>
            
            {/* System Warnings */}
            {status.system_warnings.length > 0 && (
              <Alert>
                <AlertTriangle className="h-4 w-4" />
                <AlertDescription>
                  <ul className="list-disc list-inside text-sm">
                    {status.system_warnings.map((warning, index) => (
                      <li key={index}>{warning}</li>
                    ))}
                  </ul>
                </AlertDescription>
              </Alert>
            )}
          </CardContent>
        </Card>
      )}

      {/* Configuration Tabs */}
      <Tabs defaultValue="sales" className="space-y-4">
        <TabsList className="grid w-full grid-cols-2">
          <TabsTrigger value="sales">Sales Transaction Accounts</TabsTrigger>
          <TabsTrigger value="revenue">Revenue & Tax Accounts</TabsTrigger>
        </TabsList>
        
        <TabsContent value="sales" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center space-x-2">
                <BookOpen className="h-4 w-4" />
                <span>Sales Transaction Accounts</span>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {status && (
                <>
                  <AccountSelector
                    role="sales_receivable"
                    label="Receivable Account"
                    description="Used for credit sales (Piutang Usaha)"
                    accountStatus={status.sales_receivable}
                    expectedType="ASSET"
                  />
                  <Separator />
                  <AccountSelector
                    role="sales_cash"
                    label="Cash Account"
                    description="Used for cash sales"
                    accountStatus={status.sales_cash}
                    expectedType="ASSET"
                  />
                  <Separator />
                  <AccountSelector
                    role="sales_bank"
                    label="Bank Account"
                    description="Used for bank transfer sales"
                    accountStatus={status.sales_bank}
                    expectedType="ASSET"
                  />
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>
        
        <TabsContent value="revenue" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center space-x-2">
                <Eye className="h-4 w-4" />
                <span>Revenue & Tax Accounts</span>
              </CardTitle>
            </CardHeader>
            <CardContent className="space-y-4">
              {status && (
                <>
                  <AccountSelector
                    role="sales_revenue"
                    label="Revenue Account"
                    description="Main revenue account for sales"
                    accountStatus={status.sales_revenue}
                    expectedType="REVENUE"
                  />
                  <Separator />
                  <AccountSelector
                    role="sales_output_vat"
                    label="Output VAT Account"
                    description="PPN Keluaran - sales tax obligation"
                    accountStatus={status.sales_output_vat}
                    expectedType="LIABILITY"
                  />
                </>
              )}
            </CardContent>
          </Card>
        </TabsContent>
      </Tabs>
    </div>
  );
};

export default EnhancedTaxAccountSettings;
