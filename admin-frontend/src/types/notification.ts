export const nrequestMethods: Record<number, string> = {
    1: "GET",
    2: "POST",
}

export const nrequestTypes: Record<number, string> = {
    1: "JSON",
    2: "Form",
}

// 与后端 model.NotificationChannel* 编号保持一致：0=webhook 1=钉钉 2=Server酱
export const nChannels: Record<number, string> = {
    0: "Webhook",
    1: "DingTalk",
    2: "ServerChan",
}
