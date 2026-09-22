package com.github.kr328.clash

import android.Manifest.permission.INTERNET
import android.content.ClipData
import android.content.ClipboardManager
import android.content.pm.ApplicationInfo
import android.content.pm.PackageInfo
import android.content.pm.PackageManager
import androidx.core.content.getSystemService
import com.github.kr328.clash.design.AccessControlDesign
import com.github.kr328.clash.design.model.AppInfo
import com.github.kr328.clash.design.util.toAppInfo
import com.github.kr328.clash.service.store.ServiceStore
import com.github.kr328.clash.util.startClashService
import com.github.kr328.clash.util.stopClashService
import com.github.kr328.clash.util.AccessControlTransfer
import com.github.kr328.clash.design.ui.ToastDuration
import kotlinx.coroutines.CancellationException
import com.github.kr328.clash.design.R as DesignR
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.delay
import kotlinx.coroutines.isActive
import kotlinx.coroutines.selects.select
import kotlinx.coroutines.withContext

class AccessControlActivity : BaseActivity<AccessControlDesign>() {
    override suspend fun main() {
        val service = ServiceStore(this)
        var mode = service.accessControlMode

        val selected = withContext(Dispatchers.IO) {
            service.accessControlPackages.toMutableSet()
        }

        defer {
            withContext(Dispatchers.IO) {
                val changed = selected != service.accessControlPackages || mode != service.accessControlMode
                service.accessControlPackages = selected
                service.accessControlMode = mode
                if (clashRunning && changed) {
                    stopClashService()
                    while (clashRunning) {
                        delay(200)
                    }
                    startClashService()
                }
            }
        }

        val design = AccessControlDesign(this, uiStore, selected)

        setContentDesign(design)

        design.requests.send(AccessControlDesign.Request.ReloadApps)

        while (isActive) {
            select<Unit> {
                events.onReceive {

                }
                design.requests.onReceive {
                    when (it) {
                        AccessControlDesign.Request.ReloadApps -> {
                            design.patchApps(loadApps(selected))
                        }

                        AccessControlDesign.Request.SelectAll -> {
                            val all = withContext(Dispatchers.Default) {
                                design.apps.map(AppInfo::packageName)
                            }

                            selected.clear()
                            selected.addAll(all)

                            design.rebindAll()
                        }

                        AccessControlDesign.Request.SelectNone -> {
                            selected.clear()

                            design.rebindAll()
                        }

                        AccessControlDesign.Request.SelectInvert -> {
                            val all = withContext(Dispatchers.Default) {
                                design.apps.map(AppInfo::packageName).toSet() - selected
                            }

                            selected.clear()
                            selected.addAll(all)

                            design.rebindAll()
                        }

                        AccessControlDesign.Request.Import -> {
                            try {
                                val clip = getSystemService<ClipboardManager>()?.primaryClip
                                val text = if (clip != null && clip.itemCount > 0) clip.getItemAt(0).text?.toString().orEmpty() else ""
                                val backup = withContext(Dispatchers.Default) { AccessControlTransfer.decode(text) }
                                val imported = backup.packages - packageName
                                val missing = withContext(Dispatchers.IO) {
                                    imported.count { name ->
                                        try { packageManager.getApplicationInfo(name, 0); false }
                                        catch (_: PackageManager.NameNotFoundException) { true }
                                    }
                                }
                                val apps = loadApps(imported)
                                selected.clear()
                                selected.addAll(imported)
                                mode = backup.mode ?: mode
                                design.patchApps(apps)
                                design.showToast(getString(DesignR.string.access_control_imported, selected.size, missing), ToastDuration.Long)
                            } catch (e: CancellationException) { throw e
                            } catch (e: Exception) {
                                design.showToast(getString(DesignR.string.access_control_import_failed, e.message ?: e.toString()), ToastDuration.Long)
                            }
                        }

                        AccessControlDesign.Request.Export -> {
                            try {
                                val clipboard = checkNotNull(getSystemService<ClipboardManager>())
                                clipboard.setPrimaryClip(ClipData.newPlainText("CMFA application routing", AccessControlTransfer.encode(selected, mode)))
                                design.showToast(DesignR.string.access_control_exported, ToastDuration.Short)
                            } catch (e: Exception) {
                                design.showToast(e.message ?: e.toString(), ToastDuration.Long)
                            }
                        }
                    }
                }
            }
        }
    }

    private suspend fun loadApps(selected: Set<String>): List<AppInfo> =
        withContext(Dispatchers.IO) {
            val reverse = uiStore.accessControlReverse
            val sort = uiStore.accessControlSort
            val systemApp = uiStore.accessControlSystemApp

            val base = compareByDescending<AppInfo> { it.packageName in selected }
            val comparator = if (reverse) base.thenDescending(sort) else base.then(sort)

            val pm = packageManager
            val packages = pm.getInstalledPackages(PackageManager.GET_PERMISSIONS)

            packages.asSequence()
                .filter {
                    it.packageName != packageName
                }
                .filter {
                    it.applicationInfo != null
                }
                .filter {
                    it.requestedPermissions?.contains(INTERNET) == true || it.applicationInfo!!.uid < android.os.Process.FIRST_APPLICATION_UID
                }
                .filter {
                    systemApp || !it.isSystemApp || it.packageName in selected
                }
                .map {
                    it.toAppInfo(pm)
                }
                .sortedWith(comparator)
                .toList()
        }

    private val PackageInfo.isSystemApp: Boolean
        get() {
            return applicationInfo?.flags?.and(ApplicationInfo.FLAG_SYSTEM) != 0
        }
}
