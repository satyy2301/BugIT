package com.bugit.dre

import com.intellij.openapi.project.Project
import com.intellij.openapi.wm.ToolWindow
import com.intellij.openapi.wm.ToolWindowFactory
import com.intellij.ui.components.JBScrollPane
import com.intellij.ui.content.ContentFactory
import java.awt.BorderLayout
import javax.swing.JLabel
import javax.swing.JPanel
import javax.swing.JTable
import javax.swing.table.DefaultTableModel

class DreToolWindowFactory : ToolWindowFactory {
    override fun createToolWindowContent(project: Project, toolWindow: ToolWindow) {
        val panel = JPanel(BorderLayout())
        val title = JLabel(DreState.title)
        val model = DefaultTableModel(arrayOf("#", "Service", "Summary", "Error"), 0)
        val table = JTable(model)
        refresh(model)
        panel.add(title, BorderLayout.NORTH)
        panel.add(JBScrollPane(table), BorderLayout.CENTER)
        val content = ContentFactory.getInstance().createContent(panel, "", false)
        toolWindow.contentManager.addContent(content)
    }

    private fun refresh(model: DefaultTableModel) {
        model.rowCount = 0
        for (e in DreState.events) {
            model.addRow(arrayOf(e.index + 1, e.service, e.summary, if (e.isError) "yes" else ""))
        }
    }
}
