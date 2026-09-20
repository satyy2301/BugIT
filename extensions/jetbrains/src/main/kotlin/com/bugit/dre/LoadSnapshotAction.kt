package com.bugit.dre

import com.intellij.openapi.actionSystem.AnAction
import com.intellij.openapi.actionSystem.AnActionEvent
import com.intellij.openapi.fileChooser.FileChooser
import com.intellij.openapi.fileChooser.FileChooserDescriptor
import com.intellij.openapi.ui.Messages
import java.io.File
import java.nio.charset.StandardCharsets

class LoadSnapshotAction : AnAction() {
    override fun actionPerformed(e: AnActionEvent) {
        val project = e.project
        val descriptor = FileChooserDescriptor(true, false, false, false, false, false)
            .withTitle("Select DRE snapshot")
            .withFileFilter { it.extension?.equals("dre", ignoreCase = true) == true }
        val vf = FileChooser.chooseFile(descriptor, project, null) ?: return
        val drePath = vf.path
        val replayBin = resolveReplayBin()
        if (!File(replayBin).exists()) {
            Messages.showErrorDialog(project, "dre-replay not found at $replayBin", "DRE Load")
            return
        }
        try {
            val proc = ProcessBuilder(replayBin, "load", "--format", "ide", "--dre", drePath)
                .redirectErrorStream(true)
                .start()
            val out = proc.inputStream.bufferedReader(StandardCharsets.UTF_8).readText()
            val code = proc.waitFor()
            if (code != 0) {
                Messages.showErrorDialog(project, out.ifBlank { "exit $code" }, "dre-replay failed")
                return
            }
            DreState.applyIdeJson(out)
            Messages.showInfoMessage(project, "Loaded: ${DreState.title}", "DRE")
            project?.let { com.intellij.openapi.wm.ToolWindowManager.getInstance(it).getToolWindow("DRE Replay")?.activate(null) }
        } catch (ex: Exception) {
            Messages.showErrorDialog(project, ex.message ?: ex.toString(), "DRE Load")
        }
    }

    private fun resolveReplayBin(): String {
        val env = System.getenv("DRE_REPLAY_BIN")
        if (!env.isNullOrBlank() && File(env).exists()) return env
        val cwd = System.getProperty("user.dir")
        val win = File(cwd, "bin/dre-replay.exe")
        if (win.exists()) return win.absolutePath
        val unix = File(cwd, "bin/dre-replay")
        if (unix.exists()) return unix.absolutePath
        return "dre-replay"
    }
}
