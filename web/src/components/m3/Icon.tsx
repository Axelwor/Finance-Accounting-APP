/**
 * Pure SVG icon component using lucide-react mappings.
 */
import type { ComponentType, CSSProperties } from "react";
import {
  Wallet,
  Tag,
  ShoppingCart,
  Package,
  Building2,
  FileText,
  BookOpen,
  Factory,
  Mail,
  Receipt,
  FileCode,
  Check,
  X,
  AlertTriangle,
  AlertCircle,
  Info,
  ChevronDown,
  ChevronRight,
  ChevronUp,
  Plus,
  Trash2,
  Edit2,
  Printer,
  Copy,
  RefreshCw,
  Search,
  Settings,
  LogOut,
  Moon,
  Sun,
  Palette,
  ArrowRight,
  ArrowLeft,
  ArrowUpRight,
  ArrowDownLeft,
  DollarSign,
  TrendingUp,
  TrendingDown,
  ShieldCheck,
  Clock,
  Layers,
  Upload,
  Download,
  Calendar,
  Lock,
  Unlock,
  CheckCircle2,
  HelpCircle,
  Paperclip,
  Eye,
  SlidersHorizontal,
  ExternalLink,
  Save,
  Send,
  Building,
  User,
  Users,
  CreditCard,
  Percent,
  FileSpreadsheet
} from "lucide-react";

const ICON_MAP: Record<string, ComponentType<{ size?: number | string; className?: string; strokeWidth?: number; style?: CSSProperties }>> = {
  // Navigation & modules
  account_balance_wallet: Wallet,
  wallet: Wallet,
  sell: Tag,
  sale: Tag,
  tag: Tag,
  shopping_cart: ShoppingCart,
  purchase: ShoppingCart,
  inventory_2: Package,
  box: Package,
  apartment: Building2,
  building: Building,
  description: FileText,
  report: FileText,
  menu_book: BookOpen,
  ledger: BookOpen,
  factory: Factory,
  mail: Mail,
  email: Mail,
  receipt_long: Receipt,
  receipt: Receipt,
  file_code: FileCode,

  // Common UI actions & states
  book_open: BookOpen,
  arrow_down_left: ArrowDownLeft,
  arrow_up_right: ArrowUpRight,
  package: Package,
  check: Check,
  close: X,
  x: X,
  warning: AlertTriangle,
  error: AlertCircle,
  info: Info,
  expand_more: ChevronDown,
  chevron_down: ChevronDown,
  chevron_right: ChevronRight,
  chevron_up: ChevronUp,
  add: Plus,
  plus: Plus,
  delete: Trash2,
  trash: Trash2,
  edit: Edit2,
  print: Printer,
  printer: Printer,
  content_copy: Copy,
  copy: Copy,
  refresh: RefreshCw,
  search: Search,
  settings: Settings,
  logout: LogOut,
  exit_to_app: LogOut,
  dark_mode: Moon,
  light_mode: Sun,
  palette: Palette,
  arrow_forward: ArrowRight,
  arrow_back: ArrowLeft,
  trending_up: TrendingUp,
  trending_down: TrendingDown,
  security: ShieldCheck,
  schedule: Clock,
  layers: Layers,
  upload_file: Upload,
  download: Download,
  calendar_today: Calendar,
  lock: Lock,
  lock_open: Unlock,
  check_circle: CheckCircle2,
  help: HelpCircle,
  attach_file: Paperclip,
  visibility: Eye,
  tune: SlidersHorizontal,
  open_in_new: ExternalLink,
  save: Save,
  send: Send,
  person: User,
  people: Users,
  credit_card: CreditCard,
  percent: Percent,
  currency_exchange: DollarSign,
  table_chart: FileSpreadsheet,
};

export interface IconProps {
  /** Lookup key resolved via ICON_MAP; unknown names fall back to FileText. */
  name: string;
  slot?: string;
  filled?: boolean;
  size?: number | string;
  strokeWidth?: number;
  className?: string;
  style?: CSSProperties;
}

export function Icon({ name, slot, size = 18, strokeWidth = 1.75, className, style }: IconProps) {
  const normalizedKey = name.toLowerCase().trim();
  const LucideComponent = ICON_MAP[normalizedKey] || FileText;

  const parsedSize = typeof size === "number" ? size : parseInt(size, 10) || 18;

  return (
    <span
      slot={slot}
      className={`inline-flex items-center justify-center pure-svg-icon ${className || ""}`}
      style={{
        display: "inline-flex",
        alignItems: "center",
        justifyContent: "center",
        verticalAlign: "middle",
        width: parsedSize,
        height: parsedSize,
        ...style
      }}
      aria-hidden="true"
    >
      <LucideComponent size={parsedSize} strokeWidth={strokeWidth} />
    </span>
  );
}
