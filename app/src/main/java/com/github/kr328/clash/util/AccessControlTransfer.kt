package com.github.kr328.clash.util

import com.github.kr328.clash.service.model.AccessControlMode
import kotlinx.serialization.json.*

/** Portable package names, never device-specific UIDs. */
object AccessControlTransfer {
    data class Backup(val packages: Set<String>, val mode: AccessControlMode?)
    private val packagePattern = Regex("[A-Za-z_][A-Za-z0-9_]*(\\.[A-Za-z_][A-Za-z0-9_]*)*")
    private val json = Json { prettyPrint = true }

    fun encode(packages: Set<String>, mode: AccessControlMode): String =
        json.encodeToString(JsonObject.serializer(), buildJsonObject {
            put("format", "cmfa-access-control")
            put("version", 1)
            put("mode", mode.name)
            put("packages", buildJsonArray { packages.sorted().forEach { add(it) } })
        })

    fun decode(text: String): Backup {
        require(text.length <= 1024 * 1024) { "配置超过 1 MiB" }
        val input = text.trim().removePrefix("\uFEFF").trim()
        require(input.isNotEmpty()) { "剪贴板为空" }
        val names: List<String>
        val mode: AccessControlMode?
        if (input.startsWith("{")) {
            val obj = json.parseToJsonElement(input).jsonObject
            require(obj["format"]?.jsonPrimitive?.content == "cmfa-access-control") { "不是分应用路由备份" }
            require(obj["version"]?.jsonPrimitive?.intOrNull == 1) { "不支持此备份版本" }
            mode = AccessControlMode.values().firstOrNull { it.name == obj["mode"]?.jsonPrimitive?.content }
            require(mode != null) { "路由模式无效" }
            names = obj.getValue("packages").jsonArray.map {
                require(it is JsonPrimitive && it.isString) { "应用包名必须是文本" }
                it.content
            }
        } else {
            mode = null // Legacy one-package-per-line lists preserve the current mode.
            names = input.lineSequence().map(String::trim).filter(String::isNotEmpty).toList()
        }
        require(names.size <= 10000) { "应用名单最多 10000 项" }
        require(names.all { it.length <= 255 && packagePattern.matches(it) }) { "存在无效应用包名" }
        return Backup(names.toSet(), mode)
    }
}
