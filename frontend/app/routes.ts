
import { type RouteConfig, index, route } from "@react-router/dev/routes";

export default [
  route("admin/login", "routes/admin/login.tsx"), // 管理者用ログイン
  route("admin", "routes/admin/dashboard.tsx"),
  route("register", "routes/customer/register.tsx"), // 顧客登録をトップレベルに統一
  route("forgot-password", "routes/customer/forgot-password.tsx"),
  route("reset-password", "routes/customer/reset-password.tsx"),
  route("change-password", "routes/customer/change-password.tsx"),
  index("routes/home.tsx"),
  route("provider", "routes/provider.tsx", [
    route("login", "routes/provider/login.tsx"), // プロバイダー用ログイン
    index("routes/provider/index.tsx"),
    route("set-password", "routes/provider/set-password.tsx"),
    route("apis", "routes/provider/apis.tsx"),
    route("apis/new", "routes/provider/apis/new.tsx"),
    route("apis/:id/edit", "routes/provider/apis/[id]/edit.tsx"),
    route("change-password", "routes/provider/change-password.tsx"),
    route("forgot-password", "routes/provider/forgot-password.tsx"),
  ]),
  route("*", "routes/catch-all.tsx"),
] satisfies RouteConfig;
