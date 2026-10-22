import { Navigate, Route, Routes, useLocation } from "react-router-dom";
import { useTranslation } from "react-i18next";
import { DataSources } from "./components/DataSources";
import { Shell } from "./Shell";
import { LoginPage } from "./pages/Login";
import { DashboardPage } from "./pages/Dashboard";
import { TradingPage } from "./pages/Trading";
import { ResourceTable } from "./pages/ResourceTable";
import { KFBPage } from "./pages/KFB";
import { LoadOrderTrackingPage } from "./pages/LoadOrderTracking";
import { DepositoryPage } from "./pages/Depository";
import { UsersPage } from "./pages/Users";
import { ClientsPage } from "./pages/Clients";
import { ContractsPage } from "./pages/Contracts";
import { AccountsPage } from "./pages/Accounts";
import { PositionsPage } from "./pages/Positions";
import { ProfilePage } from "./pages/Profile";
import { WalletPage } from "./pages/Wallet";
import { PaymentsPage } from "./pages/Payments";
import { TradesPage } from "./pages/Trades";
import { LoadTestingPage } from "./pages/LoadTesting";
import { AuthProvider, useAuth } from "./auth";
import { LandingPage } from "./pages/Landing";
import { MobileDownloadPage } from "./pages/MobileDownload";
import { MobileAppPrompt } from "./components/MobileAppPrompt";

function Guard({ children }: { children: JSX.Element }) {
  const { authenticated } = useAuth();
  const { pathname } = useLocation();
  if (!authenticated && pathname === "/") return <LandingPage />;
  if (!authenticated) return <Navigate to="/login" replace />;
  return children;
}

function T({ k, path, cols }: { k: string; path: string; cols: { key: string; label: string }[] }) {
  const { t } = useTranslation();
  return <ResourceTable title={t(k)} path={path} columns={cols} />;
}

export default function App() {
  return (
    <AuthProvider>
      <MobileAppPrompt />
      <Routes>
        <Route path="/welcome" element={<LandingPage />} />
        <Route path="/download" element={<MobileDownloadPage />} />
        <Route path="/login" element={<div className="login-route"><div className="px-5 pt-5"><DataSources /></div><LoginPage /></div>} />
        <Route
          path="/"
          element={
            <Guard>
              <Shell />
            </Guard>
          }
        >
          <Route index element={<DashboardPage />} />
          <Route path="profile" element={<ProfilePage />} />
          <Route path="trading" element={<TradingPage />} />
          <Route path="order-book" element={<TradingPage />} />
          <Route path="orders" element={<T k="nav.orders" path="/v1/orders" cols={[{ key: "client_order_id", label: "ID" }, { key: "side", label: "Side" }, { key: "status", label: "Status" }, { key: "quantity", label: "Qty" }]} />} />
          <Route path="trades" element={<TradesPage />} />
          <Route path="clients" element={<ClientsPage />} />
          <Route path="kyc" element={<T k="nav.kyc" path="/v1/clients" cols={[{ key: "client_no", label: "No" }, { key: "kyc_status", label: "KYC" }, { key: "risk_level", label: "Risk" }]} />} />
          <Route path="documents" element={<T k="nav.documents" path="/v1/clients" cols={[{ key: "client_no", label: "Client" }, { key: "status", label: "Status" }]} />} />
          <Route path="contracts" element={<ContractsPage />} />
          <Route path="accounts" element={<AccountsPage />} />
          <Route path="cash" element={<WalletPage />} />
          <Route path="positions" element={<PositionsPage />} />
          <Route path="portfolio" element={<WalletPage />} />
          <Route path="settlement" element={<T k="nav.settlement" path="/v1/settlements" cols={[{ key: "id", label: "ID" }, { key: "cash_status", label: "Cash" }, { key: "securities_status", label: "Sec" }, { key: "settlement_date", label: "Date" }]} />} />
          <Route path="deposits" element={<PaymentsPage kind="deposits" />} />
          <Route path="withdrawals" element={<PaymentsPage kind="withdrawals" />} />
          <Route path="instruments" element={<T k="nav.instruments" path="/v1/instruments" cols={[{ key: "ticker", label: "Ticker" }, { key: "isin", label: "ISIN" }, { key: "name", label: "Name" }, { key: "status", label: "Status" }]} />} />
          <Route path="issuers" element={<T k="nav.issuers" path="/v1/issuers" cols={[{ key: "name", label: "Name" }, { key: "country", label: "Country" }, { key: "status", label: "Status" }]} />} />
          <Route path="corporate-actions" element={<T k="nav.corporateActions" path="/v1/corporate-actions" cols={[{ key: "type", label: "Type" }, { key: "ex_date", label: "Ex date" }, { key: "status", label: "Status" }]} />} />
          <Route path="reports" element={<T k="nav.reports" path="/v1/reports" cols={[{ key: "code", label: "Code" }, { key: "name", label: "Name" }]} />} />
          <Route path="integrations/kfb" element={<KFBPage />} />
          <Route path="integrations/depository" element={<DepositoryPage />} />
          <Route path="integrations/exchanges" element={<Navigate to="/integrations/kfb" replace />} />
          <Route path="integrations/fix" element={<Navigate to="/integrations/kfb" replace />} />
          <Route path="integrations/quik" element={<Navigate to="/integrations/kfb" replace />} />
          <Route path="integrations/banks" element={<Navigate to="/integrations/depository" replace />} />
          <Route path="brokers" element={<T k="nav.brokers" path="/v1/brokers" cols={[{ key: "code", label: "Code" }, { key: "name", label: "Name" }, { key: "status", label: "Status" }]} />} />
          <Route path="users" element={<UsersPage />} />
          <Route path="roles" element={<T k="nav.roles" path="/v1/roles" cols={[{ key: "code", label: "Code" }, { key: "name", label: "Name" }]} />} />
          <Route path="tokens" element={<T k="nav.apiTokens" path="/v1/brokers" cols={[{ key: "code", label: "Broker" }, { key: "status", label: "Status" }]} />} />
          <Route path="settings" element={<T k="nav.settings" path="/v1/roles" cols={[{ key: "code", label: "Key" }, { key: "name", label: "Value" }]} />} />
          <Route path="logs" element={<T k="nav.logs" path="/v1/audit" cols={[{ key: "action", label: "Action" }, { key: "entity_type", label: "Entity" }, { key: "created_at", label: "Time" }]} />} />
          <Route path="audit" element={<T k="nav.audit" path="/v1/audit" cols={[{ key: "action", label: "Action" }, { key: "entity_type", label: "Entity" }, { key: "created_at", label: "Time" }]} />} />
          <Route path="notifications" element={<T k="nav.notifications" path="/v1/notifications" cols={[{ key: "severity", label: "Sev" }, { key: "title", label: "Title" }, { key: "created_at", label: "Time" }]} />} />
          <Route path="health" element={<T k="nav.health" path="/v1/adapters" cols={[{ key: "code", label: "Component" }, { key: "status", label: "Status" }]} />} />
          <Route path="load-testing" element={<LoadTestingPage />} />
          <Route path="load-testing/orders/:id" element={<LoadOrderTrackingPage />} />
        </Route>
      </Routes>
    </AuthProvider>
  );
}
