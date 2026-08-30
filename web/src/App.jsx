import { BrowserRouter, Routes, Route } from 'react-router-dom'
import MainLayout from './layouts/MainLayout.jsx'
import MaterialPage from './pages/MaterialPage.jsx'
import SupplierPage from './pages/SupplierPage.jsx'
import CustomerPage from './pages/CustomerPage.jsx'
import WarehousePage from './pages/WarehousePage.jsx'
import LocationPage from './pages/LocationPage.jsx'
import PurchaseOrderPage from './pages/PurchaseOrderPage.jsx'
import SalesOrderPage from './pages/SalesOrderPage.jsx'
import StockPage from './pages/StockPage.jsx'
import InventoryPage from './pages/InventoryPage.jsx'
import PlanningPage from './pages/PlanningPage.jsx'

// Front-end router: one route per module, all rendered inside MainLayout.
export default function App() {
   return (
     <BrowserRouter>
       <Routes>
         <Route element={<MainLayout />}>
           <Route index element={<MaterialPage />} />
           <Route path="materials" element={<MaterialPage />} />
           <Route path="suppliers" element={<SupplierPage />} />
           <Route path="customers" element={<CustomerPage />} />
           <Route path="warehouses" element={<WarehousePage />} />
           <Route path="locations" element={<LocationPage />} />
           <Route path="purchase-orders" element={<PurchaseOrderPage />} />
           <Route path="sales-orders" element={<SalesOrderPage />} />
           <Route path="stock" element={<StockPage />} />
           <Route path="inventory" element={<InventoryPage />} />
           <Route path="planning" element={<PlanningPage />} />
         </Route>
       </Routes>
     </BrowserRouter>
   )
}
