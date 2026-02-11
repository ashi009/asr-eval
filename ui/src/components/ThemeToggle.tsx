import { Moon, Sun, Monitor } from "lucide-react";
import { useTheme } from "../utils/theme";

const cycle = { light: "system", system: "dark", dark: "light" } as const;
const icons = { light: Sun, system: Monitor, dark: Moon };
const labels = { light: "Light", system: "System", dark: "Dark" };

export function ThemeToggle() {
  const { theme, setTheme } = useTheme();
  const Icon = icons[theme];

  return (
    <button
      onClick={() => setTheme(cycle[theme])}
      className="p-1.5 rounded-md hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
      title={`Theme: ${labels[theme]}`}
    >
      <Icon size={16} />
    </button>
  );
}
