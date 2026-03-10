import { useCallback, useEffect, useRef, useState, type ChangeEvent } from 'react'
import {
  Alert,
  Button,
  Modal,
  ModalBody,
  ModalContent,
  ModalFooter,
  ModalHeader,
  Skeleton,
  Switch,
  Textarea,
} from '@heroui/react'
import { FolderOpen, Languages, PackagePlus, RefreshCw, Upload } from 'lucide-react'

import {
  fetchSkillSettings,
  postSkillInstallArchive,
  postSkillInstallDirectory,
  postTranslateText,
  putSkillSettings,
  type SkillBindingSetting,
  type SkillSettings,
} from '@/lib/daemon'
import { toast } from '@/lib/toast'
import { useDaemonStore } from '@/stores/daemonStore'

import { SETTINGS_PANEL_BUTTON_CLASS, SETTINGS_TEXTAREA_CLASSNAMES, SettingsTabLayout } from './settingsUi'

function normalizeSkills(skills: SkillBindingSetting[]): SkillBindingSetting[] {
  return skills.map((skill) => ({
    id: skill.id,
    description: skill.description,
    path: skill.path,
    source: skill.source,
    enabled: skill.enabled !== false,
  }))
}

export function SkillSettingsTab() {
  const daemonUrl = useDaemonStore((s) => s.daemonUrl)
  const [data, setData] = useState<SkillSettings | null>(null)
  const [loading, setLoading] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const [savingSkillID, setSavingSkillID] = useState<string | null>(null)
  const [addOpen, setAddOpen] = useState(false)
  const [installing, setInstalling] = useState(false)
  const archiveInputRef = useRef<HTMLInputElement | null>(null)
  const directoryInputRef = useRef<HTMLInputElement | null>(null)

  // 编辑弹窗状态
  const [editSkill, setEditSkill] = useState<SkillBindingSetting | null>(null)
  const [editDesc, setEditDesc] = useState('')
  const [translating, setTranslating] = useState(false)
  const [translated, setTranslated] = useState('')
  const [savingEdit, setSavingEdit] = useState(false)

  const load = useCallback(async (mode: 'initial' | 'refresh' = 'initial') => {
    if (mode === 'initial') setLoading(true)
    else setRefreshing(true)
    try {
      const res = await fetchSkillSettings(daemonUrl)
      setData({ skills: normalizeSkills(res.skills ?? []) })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '加载 Skill 设置失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      if (mode === 'initial') setLoading(false)
      else setRefreshing(false)
    }
  }, [daemonUrl])

  useEffect(() => {
    void load('initial')
  }, [load])

  useEffect(() => {
    const input = directoryInputRef.current
    if (!input) return
    input.setAttribute('webkitdirectory', '')
    input.setAttribute('directory', '')
  }, [])

  const persistSkills = useCallback(async (nextSkills: SkillBindingSetting[], successTitle?: string) => {
    const res = await putSkillSettings(daemonUrl, { skills: normalizeSkills(nextSkills) })
    setData({ skills: normalizeSkills(res.skills ?? []) })
    if (successTitle) {
      toast({ title: successTitle })
    }
  }, [daemonUrl])

  const onToggleSkill = useCallback(async (skillID: string, enabled: boolean) => {
    if (!data) return
    const previous = data.skills
    const nextSkills = previous.map((skill) => (skill.id === skillID ? { ...skill, enabled } : skill))
    setData({ skills: nextSkills })
    setSavingSkillID(skillID)
    try {
      await persistSkills(nextSkills, enabled ? 'Skill 已启用' : 'Skill 已关闭')
    } catch (err: unknown) {
      setData({ skills: previous })
      toast({
        variant: 'destructive',
        title: '保存 Skill 设置失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setSavingSkillID(null)
    }
  }, [data, persistSkills])

  const onArchiveSelected = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) return
    setInstalling(true)
    setAddOpen(false)
    try {
      const res = await postSkillInstallArchive(daemonUrl, file)
      setData({ skills: normalizeSkills(res.skills ?? []) })
      toast({ title: 'Skill 已安装' })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '安装 Skill 失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setInstalling(false)
    }
  }, [daemonUrl])

  const onDirectorySelected = useCallback(async (event: ChangeEvent<HTMLInputElement>) => {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    if (files.length === 0) return
    setInstalling(true)
    setAddOpen(false)
    try {
      const res = await postSkillInstallDirectory(
        daemonUrl,
        files.map((file) => ({
          file,
          relativePath: file.webkitRelativePath || file.name,
        })),
      )
      setData({ skills: normalizeSkills(res.skills ?? []) })
      toast({ title: 'Skill 已安装' })
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '安装 Skill 失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setInstalling(false)
    }
  }, [daemonUrl])

  const openEditModal = useCallback((skill: SkillBindingSetting) => {
    setEditSkill(skill)
    setEditDesc(skill.description ?? '')
    setTranslated('')
  }, [])

  const onTranslate = useCallback(async () => {
    if (!editSkill) return
    const source = editSkill.description ?? ''
    if (!source.trim()) {
      toast({ variant: 'destructive', title: '没有可翻译的内容' })
      return
    }
    setTranslating(true)
    try {
      const res = await postTranslateText(daemonUrl, source)
      setTranslated(res.translated)
      setEditDesc(res.translated)
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '翻译失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setTranslating(false)
    }
  }, [daemonUrl, editSkill])

  const onSaveEdit = useCallback(async () => {
    if (!editSkill || !data) return
    const previous = data.skills
    const nextSkills = previous.map((skill) =>
      skill.id === editSkill.id ? { ...skill, description: editDesc } : skill,
    )
    setSavingEdit(true)
    try {
      await persistSkills(nextSkills, '说明已保存')
      setEditSkill(null)
    } catch (err: unknown) {
      toast({
        variant: 'destructive',
        title: '保存失败',
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setSavingEdit(false)
    }
  }, [data, editDesc, editSkill, persistSkills])

  if (loading) {
    return <Skeleton className="h-64 w-full rounded-xl" />
  }

  if (!data) {
    return <Alert color="danger" title="未能加载 Skill 设置" />
  }

  return (
    <SettingsTabLayout
      footer={
        <>
          <div className="flex flex-wrap items-center gap-2">
            <div className="text-sm text-muted-foreground">当前发现 {data.skills.length} 个 Skill。</div>
            <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="flat" isLoading={refreshing} onPress={() => void load('refresh')} startContent={<RefreshCw className="h-4 w-4" />}>
              刷新列表
            </Button>
          </div>
          <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} color="primary" variant="flat" isLoading={installing} onPress={() => setAddOpen(true)} startContent={<PackagePlus className="h-4 w-4" />}>
            添加 Skill
          </Button>
        </>
      }
    >
        {data.skills.length === 0 ? (
          <div className="rounded-xl border border-dashed px-4 py-6 text-sm text-muted-foreground">
            当前没有发现任何 Skill。你可以点击右上角“添加 Skill”，或在项目目录 / 用户目录下放置 `SKILL.md`。
          </div>
        ) : (
          <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3">
            {data.skills.map((skill) => (
              <section key={skill.id} className="flex flex-col gap-3 rounded-xl border bg-card p-4" style={{aspectRatio: '3/2'}}>
                <div className="flex items-center justify-between gap-2">
                  <div className="flex min-w-0 items-center gap-2">
                    <div className="truncate text-sm font-semibold">{skill.id}</div>
                  </div>
                  <Switch
                    size="sm"
                    isSelected={skill.enabled}
                    isDisabled={savingSkillID === skill.id}
                    onValueChange={(value) => void onToggleSkill(skill.id, value)}
                  />
                </div>
                {skill.description ? (
                  <div className="line-clamp-4 flex-1 text-xs text-muted-foreground">{skill.description}</div>
                ) : (
                  <div className="flex-1 text-xs text-muted-foreground/50">暂无说明</div>
                )}
                <Button
                  size="sm"
                  radius="full"
                  className={`${SETTINGS_PANEL_BUTTON_CLASS} w-full`}
                  variant="flat"
                  onPress={() => openEditModal(skill)}
                >
                  编辑说明
                </Button>
              </section>
            ))}
          </div>
        )}


      <input
        ref={archiveInputRef}
        type="file"
        accept=".zip,application/zip"
        className="hidden"
        onChange={(event) => void onArchiveSelected(event)}
      />
      <input
        ref={directoryInputRef}
        type="file"
        multiple
        className="hidden"
        onChange={(event) => void onDirectorySelected(event)}
      />

      {/* 编辑说明弹窗 */}
      <Modal isOpen={editSkill !== null} onOpenChange={(open) => { if (!open) setEditSkill(null) }} size="lg">
        <ModalContent>
          {() => (
            <>
              <ModalHeader className="flex flex-col gap-0.5">
                <span>编辑说明</span>
                <span className="text-sm font-normal text-muted-foreground">{editSkill?.id}</span>
                {editSkill?.path ? (
                  <span className="break-all text-xs text-muted-foreground/60">{editSkill.path}</span>
                ) : null}
              </ModalHeader>
              <ModalBody className="space-y-4">
                {editSkill?.description ? (
                  <div className="space-y-1">
                    <div className="text-xs font-medium text-muted-foreground">原始描述</div>
                    <div className="rounded-lg border bg-muted/30 px-3 py-2 text-sm text-muted-foreground">
                      {editSkill.description}
                    </div>
                  </div>
                ) : null}
                <div className="space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="text-xs font-medium text-muted-foreground">自定义说明</div>
                    <Button
                      size="sm"
                      radius="full"
                      className={SETTINGS_PANEL_BUTTON_CLASS}
                      variant="flat"
                      isLoading={translating}
                      isDisabled={!editSkill?.description}
                      onPress={() => void onTranslate()}
                      startContent={!translating ? <Languages className="h-3.5 w-3.5" /> : null}
                    >
                      翻译原文
                    </Button>
                  </div>
                  {translated ? (
                    <div className="text-xs text-muted-foreground">译文已填入编辑区</div>
                  ) : null}
                  <Textarea
                    classNames={SETTINGS_TEXTAREA_CLASSNAMES}
                    placeholder="输入一句话描述这个 Skill 的功能……"
                    value={editDesc}
                    onValueChange={setEditDesc}
                    minRows={3}
                    maxRows={6}
                  />
                </div>
              </ModalBody>
              <ModalFooter>
                <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="light" onPress={() => setEditSkill(null)}>取消</Button>
                <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} color="primary" isLoading={savingEdit} onPress={() => void onSaveEdit()}>保存</Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>

      {/* 添加 Skill 弹窗 */}
      <Modal isOpen={addOpen} onOpenChange={setAddOpen} size="lg">
        <ModalContent>
          {() => (
            <>
              <ModalHeader>添加 Skill</ModalHeader>
              <ModalBody className="space-y-3">
                <div className="text-sm text-muted-foreground">
                  你可以上传一个 zip 压缩包，或者直接选择一个 Skill 文件夹。安装后会放到用户目录的 `~/.codex/skills` 下。
                </div>
                <div className="grid gap-3 sm:grid-cols-2">
                  <Button radius="full" size="sm" className={`${SETTINGS_PANEL_BUTTON_CLASS} justify-start`} variant="flat" onPress={() => archiveInputRef.current?.click()} startContent={<Upload className="h-4 w-4" />}>
                    上传 ZIP 压缩包
                  </Button>
                  <Button radius="full" size="sm" className={`${SETTINGS_PANEL_BUTTON_CLASS} justify-start`} variant="flat" onPress={() => directoryInputRef.current?.click()} startContent={<FolderOpen className="h-4 w-4" />}>
                    选择 Skill 文件夹
                  </Button>
                </div>
              </ModalBody>
              <ModalFooter>
                <Button radius="full" size="sm" className={SETTINGS_PANEL_BUTTON_CLASS} variant="light" onPress={() => setAddOpen(false)}>关闭</Button>
              </ModalFooter>
            </>
          )}
        </ModalContent>
      </Modal>
    </SettingsTabLayout>
  )
}
