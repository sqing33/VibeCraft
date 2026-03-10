import { useState } from "react";
import { Settings2 } from "lucide-react";
import {
  Button,
  Modal,
  ModalBody,
  ModalContent,
  ModalHeader,
  Tab,
  Tabs,
} from "@heroui/react";

import { BasicSettingsTab } from "./BasicSettingsTab";
import { CLIToolSettingsTab } from "./CLIToolSettingsTab";
import { APISourceSettingsTab } from "./APISourceSettingsTab";
import { RuntimeModelSettingsTab } from "./RuntimeModelSettingsTab";
import { ExpertSettingsTab } from "./ExpertSettingsTab";
import { MCPSettingsTab } from "./MCPSettingsTab";
import { SkillSettingsTab } from "./SkillSettingsTab";
import {
  SETTINGS_TABS_CLASSNAMES,
} from "./settingsUi";

export function SettingsDialog() {

  const [open, setOpen] = useState(false);
  return (
    <>
      <Button
        radius="full"
        variant="light"
        size="sm"
        isIconOnly
        aria-label="设置"
        onPress={() => setOpen(true)}
      >
        <Settings2 className="h-4 w-4" aria-hidden="true" focusable="false" />
      </Button>

      <Modal
        isOpen={open}
        onOpenChange={setOpen}
        size="5xl"
        scrollBehavior="inside"
        classNames={{
          base: "h-[80vh] max-h-[85vh] min-h-0",
        }}
      >
        <ModalContent className="h-full min-h-0 overflow-hidden">
          {() => (
            <>
              <ModalHeader>系统设置</ModalHeader>
              <ModalBody className="flex min-h-0 flex-1 overflow-hidden">
                <Tabs
                  defaultSelectedKey="basic"
                  aria-label="系统设置"
                  variant="solid"
                  color="primary"
                  classNames={SETTINGS_TABS_CLASSNAMES}
                >
                  <Tab key="basic" title="基本设置">
                    <BasicSettingsTab />
                  </Tab>

                  <Tab key="api-sources" title="API 来源">
                    <APISourceSettingsTab />
                  </Tab>

                  <Tab key="runtime-models" title="模型设置">
                    <RuntimeModelSettingsTab />
                  </Tab>

                  <Tab key="cli-tools" title="CLI 工具">
                    <CLIToolSettingsTab />
                  </Tab>

                  <Tab key="mcp" title="MCP">
                    <MCPSettingsTab />
                  </Tab>

                  <Tab key="skills" title="技能">
                    <SkillSettingsTab />
                  </Tab>

                  <Tab key="experts" title="专家">
                    <ExpertSettingsTab />
                  </Tab>
                </Tabs>
              </ModalBody>
            </>
          )}
        </ModalContent>
      </Modal>
    </>
  );
}
