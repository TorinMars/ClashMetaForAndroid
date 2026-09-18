package com.github.kr328.clash

import android.content.Intent
import android.content.ClipData
import android.content.ClipboardManager
import android.content.Context
import android.os.Build
import android.provider.Settings
import android.security.KeyChain
import android.util.Base64
import android.widget.ScrollView
import android.widget.TextView
import androidx.activity.result.contract.ActivityResultContracts
import androidx.appcompat.app.AlertDialog
import com.github.kr328.clash.design.CaptureDesign
import com.github.kr328.clash.design.ui.ToastDuration
import com.github.kr328.clash.util.withClash
import com.github.kr328.clash.service.store.ServiceStore
import kotlinx.coroutines.CancellationException
import kotlinx.coroutines.Dispatchers
import com.github.kr328.clash.common.util.ticker
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import kotlinx.coroutines.selects.select
import kotlinx.coroutines.withContext
import org.json.JSONArray
import org.json.JSONObject
import java.text.DateFormat
import java.util.Date
import com.github.kr328.clash.design.R as DesignR

class CaptureActivity : BaseActivity<CaptureDesign>() {
    private val selected = mutableSetOf<String>()
    private suspend fun command(name: String, payload: String = ""): JSONObject {
        val result = JSONObject(withClash { capture(name, payload) })
        if (result.has("error")) error(result.getString("error"))
        return result
    }
    private fun JSONArray?.strings(): List<String> = if (this == null) emptyList() else (0 until length()).map { getString(it) }
    private fun lines(text: CharSequence) = text.toString().lines().map { it.trim() }.filter { it.isNotEmpty() }.distinct()

    override suspend fun main() {
        val page = CaptureDesign(this)
        setContentDesign(page)
        val state = command("state").getJSONObject("config")
        page.enabled.isChecked = state.optBoolean("enabled")
        page.https.isChecked = state.optBoolean("https")
        page.tls12Only.isChecked = state.optBoolean("tls12Only")
        page.domains.setText(state.optJSONArray("domains").strings().joinToString("\n"))
        page.paths.setText(state.optJSONArray("paths").strings().joinToString("\n"))
        selected.addAll(state.optJSONArray("packages").strings())
        page.setApps(selected.size)
        refresh(page)
        val ticks = ticker(3000)
        try {
            while (isActive) {
                try {
                    select<Unit> {
                        events.onReceive { }
                        ticks.onReceive { if (activityStarted) refresh(page) }
                        page.requests.onReceive { request ->
                            when (request) {
                                CaptureDesign.Request.Save -> {
                                    if (page.enabled.isChecked && (!clashRunning || !uiStore.enableVpn)) {
                                        error(getString(DesignR.string.capture_need_vpn))
                                    }
                                    val config = JSONObject().put("enabled", page.enabled.isChecked)
                                        .put("https", page.https.isChecked)
                                        .put("tls12Only", page.tls12Only.isChecked)
                                        .put("domains", JSONArray(lines(page.domains.text)))
                                        .put("paths", JSONArray(lines(page.paths.text)))
                                        .put("packages", JSONArray(selected.sorted()))
                                    command("configure", config.toString())
                                    page.showToast(DesignR.string.capture_saved, ToastDuration.Short)
                                    refresh(page)
                                }
                                CaptureDesign.Request.Stop -> {
                                    command("stop")
                                    page.enabled.isChecked = false
                                    refresh(page)
                                }
                                CaptureDesign.Request.Apps -> chooseApps(page)
                                CaptureDesign.Request.ExportCA -> export("cmfa-capture-ca.crt", "application/x-x509-ca-cert", command("ca").getString("pem"))
                                CaptureDesign.Request.InstallCA -> installCA(page)
                                CaptureDesign.Request.Refresh -> refresh(page)
                                CaptureDesign.Request.Clear -> { command("clear"); refresh(page) }
                                CaptureDesign.Request.CopyDiagnostics -> copyDiagnostics(page)
                                is CaptureDesign.Request.Detail -> details(command("get", request.id.toString()))
                                is CaptureDesign.Request.Menu -> showRecordMenu(request.id, page)
                            }
                        }
                    }
                } catch (e: CancellationException) { throw e
                } catch (e: Exception) { page.showToast(e.message ?: e.toString(), ToastDuration.Long) }
            }
        } finally { ticks.cancel() }
    }

    private suspend fun refresh(page: CaptureDesign) {
        val result = command("list")
        val rows = result.getJSONArray("records")
        val labels = (0 until rows.length()).map { i ->
            val r = rows.getJSONObject(i)
            val whenText = DateFormat.getTimeInstance().format(Date(r.getLong("time")))
            r.getLong("id") to "$whenText · ${r.getString("method")} · ${r.getInt("status")} · UID ${r.getInt("uid")}\n${r.getString("url")}"
        }
        page.showRecords(labels)
        val status = getString(if (result.optBoolean("enabled")) DesignR.string.capture_running else DesignR.string.capture_stopped)
        page.status("$status · ${rows.length()}/100\n${result.optString("lastError")}")
        page.diagnostics(result.optString("diagnostics"))
    }

    private fun showRecordMenu(id: Long, page: CaptureDesign) {
        AlertDialog.Builder(this).setTitle(DesignR.string.capture_detail)
            .setItems(arrayOf(getString(DesignR.string.capture_copy_curl))) { _, _ ->
                launch {
                    try {
                        val value = command("curl", id.toString()).getString("curl")
                        (getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager)
                            .setPrimaryClip(ClipData.newPlainText("cURL", value))
                        page.showToast(DesignR.string.capture_curl_copied, ToastDuration.Short)
                    } catch (e: CancellationException) { throw e
                    } catch (e: Exception) { page.showToast(e.message ?: e.toString(), ToastDuration.Long) }
                }
            }.show()
    }

    private suspend fun copyDiagnostics(page: CaptureDesign) {
        val report = command("diagnostics")
        val store = ServiceStore(this)
        val config = report.getJSONObject("config")
        val packages = config.optJSONArray("packages").strings()
        val apps = withContext(Dispatchers.IO) {
            packages.joinToString("\n") { name ->
                try {
                    val info = packageManager.getPackageInfo(name, 0)
                    val app = packageManager.getApplicationInfo(name, 0)
                    "$name · ${app.loadLabel(packageManager)} · UID ${app.uid} · version ${info.versionName}"
                } catch (e: Exception) { "$name · ${e.javaClass.simpleName}" }
            }
        }
        val log = buildString {
            appendLine("CMFA capture diagnostics")
            appendLine("Copied: ${Date()} · ${java.util.TimeZone.getDefault().id}")
            appendLine("App: ${BuildConfig.VERSION_NAME} (${BuildConfig.VERSION_CODE}) · $packageName")
            appendLine("Device: ${Build.MANUFACTURER} ${Build.MODEL} · Android ${Build.VERSION.RELEASE} / SDK ${Build.VERSION.SDK_INT}")
            appendLine("Clash running: $clashRunning · VPN setting: ${uiStore.enableVpn}")
            appendLine("Saved network settings (VPN restart required to apply):")
            appendLine("systemProxy=${store.systemProxy} stack=${store.tunStackMode} ipv6=${store.allowIpv6} dnsHijacking=${store.dnsHijacking}")
            appendLine("allowBypass=${store.allowBypass} bypassPrivateNetwork=${store.bypassPrivateNetwork}")
            appendLine("accessControl=${store.accessControlMode} packages=${store.accessControlPackages.sorted()}")
            appendLine("Selected capture apps:\n$apps")
            appendLine("Capture config:\n${config.toString(2)}")
            appendLine("Completed records: ${report.optInt("count")}")
            appendLine("Last error: ${report.optString("lastError")}")
            appendLine(report.optString("log"))
            appendLine("Scope: new TUN TCP connections and TUN UDP packets while capture is enabled. Existing TCP connections and system HTTP proxy ingress are not counted. UDP 443 is not proof of QUIC. No HTTP bodies, headers, CA keys, or proxy credentials included.")
        }
        (getSystemService(Context.CLIPBOARD_SERVICE) as ClipboardManager)
            .setPrimaryClip(ClipData.newPlainText("CMFA diagnostics", log))
        page.showToast(DesignR.string.capture_diagnostics_copied, ToastDuration.Short)
    }
    private suspend fun chooseApps(page: CaptureDesign) {
        val apps = withContext(Dispatchers.IO) {
            packageManager.getInstalledApplications(0)
                .filter { it.packageName != packageName && (packageManager.checkPermission(android.Manifest.permission.INTERNET, it.packageName) == android.content.pm.PackageManager.PERMISSION_GRANTED || it.packageName in selected) }
                .map { it.packageName to "${it.loadLabel(packageManager)}\n${it.packageName}" }
                .sortedBy { it.second.lowercase() }
        }
        val working = selected.toMutableSet()
        AlertDialog.Builder(this).setTitle(DesignR.string.capture_apps)
            .setMultiChoiceItems(apps.map { it.second }.toTypedArray(), apps.map { it.first in working }.toBooleanArray()) { _, which, checked ->
                if (checked) working.add(apps[which].first) else working.remove(apps[which].first)
            }
            .setPositiveButton(android.R.string.ok) { _, _ -> selected.clear(); selected.addAll(working); page.setApps(selected.size) }
            .setNegativeButton(android.R.string.cancel, null)
            .setNeutralButton(DesignR.string.capture_all_apps) { _, _ -> selected.clear(); page.setApps(0) }
            .show()
    }
    private suspend fun installCA(page: CaptureDesign) {
        val ca = command("ca")
        if (Build.VERSION.SDK_INT < 30) {
            val der = Base64.decode(ca.getString("pem").replace("-----BEGIN CERTIFICATE-----", "").replace("-----END CERTIFICATE-----", ""), Base64.DEFAULT)
            try { startActivity(KeyChain.createInstallIntent().putExtra(KeyChain.EXTRA_CERTIFICATE, der).putExtra(KeyChain.EXTRA_NAME, "CMFA Capture")); return
            } catch (_: Exception) { }
        }
        AlertDialog.Builder(this).setTitle(DesignR.string.capture_install_ca)
            .setMessage(getString(DesignR.string.capture_ca_manual) + "\n\nSHA-256: " + ca.getString("sha256"))
            .setPositiveButton(DesignR.string.capture_security_settings) { _, _ ->
                try { startActivity(Intent(Settings.ACTION_SECURITY_SETTINGS)) } catch (e: Exception) {
                    launch { page.showToast(e.message ?: e.toString(), ToastDuration.Long) }
                }
            }.setNegativeButton(android.R.string.cancel, null).show()
    }
    private fun details(record: JSONObject) {
        val content = buildString {
            append(record.getString("method")).append(' ').append(record.getString("url")).append("\n")
            append("Status: ").append(record.getInt("status")).append(" · UID ").append(record.getInt("uid")).append("\n")
            append(record.optString("error")).append("\n\n")
            append("Request headers\n").append(record.optString("requestHeaders")).append("\nRequest body\n")
            append(body(record.optString("requestBody"), record.optString("requestHeaders")))
            if (record.optBoolean("requestTruncated")) append("\n[truncated: 8 KiB]")
            append("\n\nResponse headers\n").append(record.optString("responseHeaders")).append("\nResponse body\n")
            append(body(record.optString("responseBody"), record.optString("responseHeaders")))
            if (record.optBoolean("responseTruncated")) append("\n[truncated: 8 KiB]")
        }
        val text = TextView(this).apply { this.text = content; setTextIsSelectable(true); setPadding(24, 16, 24, 16) }
        val scroll = ScrollView(this).apply { addView(text) }
        AlertDialog.Builder(this).setTitle(DesignR.string.capture_detail).setView(scroll)
            .setPositiveButton(android.R.string.ok, null)
            .setNeutralButton(DesignR.string.capture_export_record) { _, _ -> launch {
                try { export("capture-${record.getLong("id")}.json", "application/json", record.toString(2)) }
                catch (e: CancellationException) { throw e }
                catch (e: Exception) { design?.showToast(e.message ?: e.toString(), ToastDuration.Long) }
            } }.show()
    }
    private fun body(value: String, headers: String): String {
        var bytes = Base64.decode(value, Base64.DEFAULT)
        var suffix = ""
        if (headers.lineSequence().any { it.lowercase().startsWith("content-encoding: gzip") }) {
            val preview = java.io.ByteArrayOutputStream()
            try {
                java.util.zip.GZIPInputStream(bytes.inputStream()).use { input ->
                    val buffer = ByteArray(1024)
                    while (preview.size() < 8192) {
                        val n = input.read(buffer, 0, minOf(buffer.size, 8192 - preview.size()))
                        if (n < 0) break
                        preview.write(buffer, 0, n)
                    }
                    if (preview.size() == 8192) suffix = "\n[gzip preview: 8 KiB limit]"
                }
            } catch (_: java.io.IOException) {
                suffix = "\n[incomplete gzip preview]"
            }
            if (preview.size() > 0) bytes = preview.toByteArray()
        }
        val text = bytes.toString(Charsets.UTF_8)
        return if (text.contains('\uFFFD') || text.contains('\u0000')) "[binary / compressed; Base64]\n$value" else text + suffix
    }
    private suspend fun export(name: String, mime: String, content: String) {
        val uri = startActivityForResult(ActivityResultContracts.CreateDocument(mime), name) ?: return
        withContext(Dispatchers.IO) {
            checkNotNull(contentResolver.openOutputStream(uri)).bufferedWriter().use { it.write(content) }
        }
    }
}
