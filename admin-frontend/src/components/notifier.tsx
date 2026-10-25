import { createNotification, updateNotification } from "@/api/notification"
import { Button } from "@/components/ui/button"
import { Checkbox } from "@/components/ui/checkbox"
import {
    Dialog,
    DialogClose,
    DialogContent,
    DialogDescription,
    DialogFooter,
    DialogHeader,
    DialogTitle,
    DialogTrigger,
} from "@/components/ui/dialog"
import {
    Form,
    FormControl,
    FormField,
    FormItem,
    FormLabel,
    FormMessage,
} from "@/components/ui/form"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { ScrollArea } from "@/components/ui/scroll-area"
import {
    Select,
    SelectContent,
    SelectItem,
    SelectTrigger,
    SelectValue,
} from "@/components/ui/select"
import { IconButton } from "@/components/xui/icon-button"
import { asOptionalField } from "@/lib/utils"
import { ModelNotification } from "@/types"
import { nChannels, nrequestMethods, nrequestTypes } from "@/types"
import { zodResolver } from "@hookform/resolvers/zod"
import { useEffect, useState } from "react"
import { useForm } from "react-hook-form"
import { useTranslation } from "react-i18next"
import { toast } from "sonner"
import { KeyedMutator } from "swr"
import { z } from "zod"

import { Textarea } from "./ui/textarea"

interface NotifierCardProps {
    data?: ModelNotification
    mutate: KeyedMutator<ModelNotification[]>
}

const CHANNEL_WEBHOOK = 0
const CHANNEL_DINGTALK = 1
const CHANNEL_SERVERCHAN = 2

// 凭据字段（url / dingtalk_webhook_url / serverchan_sendkey）在编辑时由后端
// 列表接口脱敏为空串，前端无法回填；留空提交表示“不修改”。因此这些字段的
// 必填校验在 onSubmit 里按是否为已有记录判断，schema 只做类型校验。
const notificationFormSchema = z.object({
    name: z.string().min(1),
    channel: z.coerce.number().int().min(0).max(2),
    url: z.string(),
    request_method: z.coerce.number().int().min(1).max(255),
    request_type: z.coerce.number().int().min(1).max(255),
    request_header: z.string(),
    request_body: z.string(),
    verify_tls: asOptionalField(z.boolean()),
    skip_check: asOptionalField(z.boolean()),
    format_metric_units: asOptionalField(z.boolean()),
    dingtalk_webhook_url: z.string(),
    dingtalk_secret: z.string(),
    serverchan_sendkey: z.string(),
})

export const NotifierCard: React.FC<NotifierCardProps> = ({ data, mutate }) => {
    const { t } = useTranslation()
    type NotificationFormInput = z.input<typeof notificationFormSchema>
    type NotificationFormData = z.output<typeof notificationFormSchema>
    type NotificationDefaults = ModelNotification &
        Partial<Pick<NotificationFormData, "skip_check">>
    const notificationDefaults: NotificationDefaults | undefined = data

    const form = useForm<NotificationFormInput, unknown, NotificationFormData>({
        resolver: zodResolver(notificationFormSchema),
        defaultValues: data
            ? {
                  name: data.name ?? "",
                  channel: data.channel ?? CHANNEL_WEBHOOK,
                  url: data.url ?? "",
                  request_method: data.request_method ?? 1,
                  request_type: data.request_type ?? 1,
                  request_header: data.request_header ?? "",
                  request_body: data.request_body ?? "",
                  verify_tls: data.verify_tls ?? false,
                  skip_check: notificationDefaults?.skip_check ?? false,
                  format_metric_units: data.format_metric_units ?? false,
                  dingtalk_webhook_url: data.dingtalk_webhook_url ?? "",
                  dingtalk_secret: data.dingtalk_secret ?? "",
                  serverchan_sendkey: data.serverchan_sendkey ?? "",
              }
            : {
                  name: "",
                  channel: CHANNEL_WEBHOOK,
                  url: "",
                  request_method: 1,
                  request_type: 1,
                  request_header: "",
                  request_body: "",
                  verify_tls: false,
                  skip_check: false,
                  format_metric_units: false,
                  dingtalk_webhook_url: "",
                  dingtalk_secret: "",
                  serverchan_sendkey: "",
              },
        resetOptions: {
            keepDefaultValues: false,
        },
    })

    const [open, setOpen] = useState(false)
    const channel = form.watch("channel")

    // 渠道切换时清掉不属于该渠道的字段，避免提交脏数据。
    useEffect(() => {
        const subscription = form.watch((_, { name: changed }) => {
            if (changed === "channel") {
                const current = form.getValues("channel")
                if (current !== CHANNEL_WEBHOOK) {
                    form.setValue("url", "")
                    form.setValue("request_header", "")
                    form.setValue("request_body", "")
                }
                if (current !== CHANNEL_DINGTALK) {
                    form.setValue("dingtalk_webhook_url", "")
                    form.setValue("dingtalk_secret", "")
                }
                if (current !== CHANNEL_SERVERCHAN) {
                    form.setValue("serverchan_sendkey", "")
                }
            }
        })
        return () => subscription.unsubscribe()
    }, [form])

    const onSubmit = async (values: NotificationFormData) => {
        // 凭据字段：新建时必填；编辑时留空=沿用旧值（后端列表已脱敏）。
        const isNew = !data?.id
        if (
            isNew &&
            ((values.channel === CHANNEL_WEBHOOK && !values.url) ||
                (values.channel === CHANNEL_DINGTALK && !values.dingtalk_webhook_url) ||
                (values.channel === CHANNEL_SERVERCHAN && !values.serverchan_sendkey))
        ) {
            toast(t("Error"), {
                description: t("Results.UnExpectedError"),
            })
            return
        }
        try {
            if (data?.id) {
                await updateNotification(data.id, values)
            } else {
                await createNotification(values)
            }
        } catch (e) {
            console.error(e)
            toast(t("Error"), {
                description: t("Results.UnExpectedError"),
            })
            return
        }
        setOpen(false)
        await mutate()
        form.reset()
    }

    const isWebhook = channel === CHANNEL_WEBHOOK
    const isDingtalk = channel === CHANNEL_DINGTALK
    const isServerchan = channel === CHANNEL_SERVERCHAN

    return (
        <Dialog open={open} onOpenChange={setOpen}>
            <DialogTrigger asChild>
                {data ? <IconButton variant="outline" icon="edit" /> : <IconButton icon="plus" />}
            </DialogTrigger>
            <DialogContent className="sm:max-w-xl">
                <ScrollArea className="max-h-[calc(100dvh-5rem)] p-3">
                    <div className="items-center mx-1">
                        <DialogHeader>
                            <DialogTitle>
                                {data ? t("EditNotifier") : t("CreateNotifier")}
                            </DialogTitle>
                            <DialogDescription />
                        </DialogHeader>
                        <Form {...form}>
                            <form onSubmit={form.handleSubmit(onSubmit)} className="space-y-2 my-2">
                                <FormField
                                    control={form.control}
                                    name="name"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel>{t("Name")}</FormLabel>
                                            <FormControl>
                                                <Input placeholder="My Notifier" {...field} />
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />
                                <FormField
                                    control={form.control}
                                    name="channel"
                                    render={({ field }) => (
                                        <FormItem>
                                            <FormLabel>{t("NotificationChannel")}</FormLabel>
                                            <Select
                                                onValueChange={(v) => field.onChange(Number(v))}
                                                value={`${field.value}`}
                                            >
                                                <FormControl>
                                                    <SelectTrigger>
                                                        <SelectValue
                                                            placeholder={t("NotificationChannel")}
                                                        />
                                                    </SelectTrigger>
                                                </FormControl>
                                                <SelectContent>
                                                    {Object.entries(nChannels).map(
                                                        ([k, v]) => (
                                                            <SelectItem key={k} value={k}>
                                                                {t(`Channel${v}`)}
                                                            </SelectItem>
                                                        ),
                                                    )}
                                                </SelectContent>
                                            </Select>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />
                                {isDingtalk && (
                                    <>
                                        <FormField
                                            control={form.control}
                                            name="dingtalk_webhook_url"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>
                                                        {t("DingtalkWebhookURL")}
                                                    </FormLabel>
                                                    <FormControl>
                                                        <Input
                                                            placeholder="https://oapi.dingtalk.com/robot/send?access_token=..."
                                                            {...field}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                        <FormField
                                            control={form.control}
                                            name="dingtalk_secret"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>
                                                        {t("DingtalkSecret")}
                                                    </FormLabel>
                                                    <FormControl>
                                                        <Input
                                                            placeholder="SEC... (optional)"
                                                            {...field}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                    <p className="text-sm text-muted-foreground">
                                                        {t("DingtalkKeywordHint")}
                                                    </p>
                                                </FormItem>
                                            )}
                                        />
                                    </>
                                )}
                                {isServerchan && (
                                    <FormField
                                        control={form.control}
                                        name="serverchan_sendkey"
                                        render={({ field }) => (
                                            <FormItem>
                                                <FormLabel>{t("ServerchanSendKey")}</FormLabel>
                                                <FormControl>
                                                    <Input placeholder="SCT..." {...field} />
                                                </FormControl>
                                                <FormMessage />
                                            </FormItem>
                                        )}
                                    />
                                )}
                                {isWebhook && (
                                    <>
                                        <FormField
                                            control={form.control}
                                            name="url"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>URL</FormLabel>
                                                    <FormControl>
                                                        <Input {...field} />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                        <FormField
                                            control={form.control}
                                            name="request_method"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("RequestMethod")}</FormLabel>
                                                    <Select
                                                        onValueChange={field.onChange}
                                                        defaultValue={`${field.value}`}
                                                    >
                                                        <FormControl>
                                                            <SelectTrigger>
                                                                <SelectValue placeholder="Request Method" />
                                                            </SelectTrigger>
                                                        </FormControl>
                                                        <SelectContent>
                                                            {Object.entries(nrequestMethods).map(
                                                                ([k, v]) => (
                                                                    <SelectItem key={k} value={k}>
                                                                        {v}
                                                                    </SelectItem>
                                                                ),
                                                            )}
                                                        </SelectContent>
                                                    </Select>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                        <FormField
                                            control={form.control}
                                            name="request_type"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("Type")}</FormLabel>
                                                    <Select
                                                        onValueChange={field.onChange}
                                                        defaultValue={`${field.value}`}
                                                    >
                                                        <FormControl>
                                                            <SelectTrigger>
                                                                <SelectValue placeholder="Request Type" />
                                                            </SelectTrigger>
                                                        </FormControl>
                                                        <SelectContent>
                                                            {Object.entries(nrequestTypes).map(
                                                                ([k, v]) => (
                                                                    <SelectItem key={k} value={k}>
                                                                        {v}
                                                                    </SelectItem>
                                                                ),
                                                            )}
                                                        </SelectContent>
                                                    </Select>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                        <FormField
                                            control={form.control}
                                            name="request_header"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("RequestHeader")}</FormLabel>
                                                    <FormControl>
                                                        <Textarea
                                                            className="resize-y"
                                                            placeholder='{"User-Agent":"Nezha-Agent"}'
                                                            {...field}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                        <FormField
                                            control={form.control}
                                            name="request_body"
                                            render={({ field }) => (
                                                <FormItem>
                                                    <FormLabel>{t("RequestBody")}</FormLabel>
                                                    <FormControl>
                                                        <Textarea
                                                            className="resize-y h-[240px]"
                                                            placeholder='{&#13;&#10;  "content":"#NEZHA#",&#13;&#10;  "ServerName":"#SERVER.NAME#",&#13;&#10;  "ServerIP":"#SERVER.IP#",&#13;&#10;  "ServerIPV4":"#SERVER.IPV4#",&#13;&#10;  "ServerIPV6":"#SERVER.IPV6#",&#13;&#10;  "CPU":"#SERVER.CPU#",&#13;&#10;  "MEM":"#SERVER.MEM#",&#13;&#10;  "SWAP":"#SERVER.SWAP#",&#13;&#10;  "DISK":"#SERVER.DISK#",&#13;&#10;  "NetInSpeed":"#SERVER.NETINSPEED#",&#13;&#10;  "NetOutSpeed":"#SERVER.NETOUTSPEED#",&#13;&#10;  "TransferIn":"#SERVER.TRANSFERIN#",&#13;&#10;  "TranferOut":"#SERVER.TRANSFEROUT#",&#13;&#10;  "Load1":"#SERVER.LOAD1#",&#13;&#10;  "Load5":"#SERVER.LOAD5#",&#13;&#10;  "Load15":"#SERVER.LOAD15#",&#13;&#10;  "TCP_CONN_COUNT":"#SERVER.TCPCONNCOUNT",&#13;&#10;  "UDP_CONN_COUNT":"#SERVER.UDPCONNCOUNT"&#13;&#10;}'
                                                            {...field}
                                                        />
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                        <FormField
                                            control={form.control}
                                            name="verify_tls"
                                            render={({ field }) => (
                                                <FormItem className="flex items-center space-x-2">
                                                    <FormControl>
                                                        <div className="flex items-center gap-2">
                                                            <Checkbox
                                                                checked={field.value === true}
                                                                onCheckedChange={field.onChange}
                                                            />
                                                            <Label className="text-sm">
                                                                {t("VerifyTLS")}
                                                            </Label>
                                                        </div>
                                                    </FormControl>
                                                    <FormMessage />
                                                </FormItem>
                                            )}
                                        />
                                    </>
                                )}
                                <FormField
                                    control={form.control}
                                    name="skip_check"
                                    render={({ field }) => (
                                        <FormItem className="flex items-center space-x-2">
                                            <FormControl>
                                                <div className="flex items-center gap-2">
                                                    <Checkbox
                                                        checked={field.value === true}
                                                        onCheckedChange={field.onChange}
                                                    />
                                                    <Label className="text-sm">
                                                        {t("DoNotSendTestMessage")}
                                                    </Label>
                                                </div>
                                            </FormControl>
                                            <FormMessage />
                                        </FormItem>
                                    )}
                                />
                                {isWebhook && (
                                    <FormField
                                        control={form.control}
                                        name="format_metric_units"
                                        render={({ field }) => (
                                            <FormItem className="flex items-center space-x-2">
                                                <FormControl>
                                                    <div className="flex items-center gap-2">
                                                        <Checkbox
                                                            checked={field.value === true}
                                                            onCheckedChange={field.onChange}
                                                        />
                                                        <Label className="text-sm">
                                                            {t("FormatMetricUnits")}
                                                        </Label>
                                                    </div>
                                                </FormControl>
                                                <FormMessage />
                                            </FormItem>
                                        )}
                                    />
                                )}
                                <DialogFooter className="justify-end">
                                    <DialogClose asChild>
                                        <Button type="button" className="my-2" variant="secondary">
                                            {t("Close")}
                                        </Button>
                                    </DialogClose>
                                    <Button type="submit" className="my-2">
                                        {t("Confirm")}
                                    </Button>
                                </DialogFooter>
                            </form>
                        </Form>
                    </div>
                </ScrollArea>
            </DialogContent>
        </Dialog>
    )
}
