import { Moon, Sun } from "lucide-react";
import { Button, Chip } from "@heroui/react";

import {
  goToChat,
  goToOrchestrations,
  goToRepoLibraryRepositories,
  useHashRoute,
} from "@/app/routes";
import { useThemeStore } from "@/stores/themeStore";

import { SettingsDialog } from "./SettingsDialog";

export function Topbar() {
  const route = useHashRoute();
  const theme = useThemeStore((s) => s.theme);
  const toggleTheme = useThemeStore((s) => s.toggleTheme);
  const isRepoLibraryRoute =
    route.name === "repo_library_repositories" ||
    route.name === "repo_library_repository_detail" ||
    route.name === "repo_library_pattern_search";

  return (
    <header className="sticky top-0 z-40 border-b bg-background/80 backdrop-blur">
      <div className="flex h-14 w-full items-center justify-between px-4">
        <div className="flex items-center gap-2">
          <div className="text-sm font-semibold tracking-tight">VibeCraft</div>
          <Chip variant="bordered" size="sm">
            前端
          </Chip>
          <Button
            variant={route.name === "chat" ? "flat" : "light"}
            size="sm"
            onPress={() => goToChat()}
          >
            Chat
          </Button>
          <Button
            variant={
              route.name === "orchestrations" ||
              route.name === "orchestration_detail"
                ? "flat"
                : "light"
            }
            size="sm"
            onPress={goToOrchestrations}
          >
            Orchestrations
          </Button>
          <Button
            variant={isRepoLibraryRoute ? "flat" : "light"}
            size="sm"
            onPress={goToRepoLibraryRepositories}
          >
            Repo Library
          </Button>
        </div>

        <div className="flex items-center gap-2">
          <Button
            variant="light"
            size="sm"
            isIconOnly
            onPress={toggleTheme}
            aria-label={theme === "dark" ? "切换为浅色主题" : "切换为深色主题"}
            title={theme === "dark" ? "切换为浅色主题" : "切换为深色主题"}
          >
            {theme === "dark" ? (
              <Sun className="h-4 w-4" aria-hidden="true" focusable="false" />
            ) : (
              <Moon className="h-4 w-4" aria-hidden="true" focusable="false" />
            )}
          </Button>
          <SettingsDialog />
        </div>
      </div>
    </header>
  );
}
