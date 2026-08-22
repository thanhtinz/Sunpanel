package com.sunpanel.app

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.outlined.Delete
import androidx.compose.material.icons.outlined.Edit
import androidx.compose.material3.Button
import androidx.compose.material3.Card
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.material3.AlertDialog
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp

/** Máy chủ đang được sửa trong hộp thoại; id rỗng nghĩa là đang thêm mới. */
private data class Draft(val id: String = "", val name: String = "", val url: String = "")

/**
 * Danh sách panel đã lưu.
 *
 * Đây là màn hình đầu tiên khi chưa có máy chủ nào được mở, và là chỗ quay về khi
 * bấm nút quay lại ở trang đầu của panel.
 */
@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ServerListScreen(
    servers: List<Server>,
    onOpen: (Server) -> Unit,
    onSave: (String, String, String) -> String?,
    onRemove: (String) -> Unit,
) {
    var draft by remember { mutableStateOf<Draft?>(null) }
    var confirmRemove by remember { mutableStateOf<Server?>(null) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.app_name)) },
                colors =
                    TopAppBarDefaults.topAppBarColors(
                        containerColor = MaterialTheme.colorScheme.surface,
                        titleContentColor = MaterialTheme.colorScheme.onSurface,
                    ),
            )
        },
        floatingActionButton = {
            FloatingActionButton(onClick = { draft = Draft() }) {
                Icon(Icons.Filled.Add, contentDescription = stringResource(R.string.server_add))
            }
        },
    ) { padding ->
        if (servers.isEmpty()) {
            EmptyState(modifier = Modifier.fillMaxSize().padding(padding))
            return@Scaffold
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            items(servers, key = { it.id }) { server ->
                ServerCard(
                    server = server,
                    onOpen = { onOpen(server) },
                    onEdit = { draft = Draft(server.id, server.name, server.url) },
                    onRemove = { confirmRemove = server },
                )
            }
        }
    }

    draft?.let { current ->
        ServerDialog(
            draft = current,
            onDismiss = { draft = null },
            onSave = { name, url ->
                val error = onSave(current.id, name, url)
                if (error == null) draft = null
                error
            },
        )
    }

    // Xóa hỏi lại một nhịp: gõ lại một địa chỉ kèm đường dẫn bí mật trên bàn phím
    // điện thoại không phải chuyện nhanh.
    confirmRemove?.let { server ->
        AlertDialog(
            onDismissRequest = { confirmRemove = null },
            title = { Text(stringResource(R.string.server_remove_title)) },
            text = { Text(stringResource(R.string.server_remove_body, server.name)) },
            confirmButton = {
                TextButton(
                    onClick = {
                        onRemove(server.id)
                        confirmRemove = null
                    }
                ) {
                    Text(stringResource(R.string.server_remove_confirm))
                }
            },
            dismissButton = {
                TextButton(onClick = { confirmRemove = null }) { Text(stringResource(R.string.cancel)) }
            },
        )
    }
}

@Composable
private fun EmptyState(modifier: Modifier = Modifier) {
    Column(
        modifier = modifier.padding(32.dp),
        verticalArrangement = Arrangement.Center,
        horizontalAlignment = Alignment.CenterHorizontally,
    ) {
        Text(
            text = stringResource(R.string.empty_title),
            style = MaterialTheme.typography.titleMedium,
        )
        Spacer(modifier = Modifier.height(8.dp))
        Text(
            text = stringResource(R.string.empty_body),
            style = MaterialTheme.typography.bodyMedium,
            color = MaterialTheme.colorScheme.onSurfaceVariant,
        )
    }
}

@Composable
private fun ServerCard(server: Server, onOpen: () -> Unit, onEdit: () -> Unit, onRemove: () -> Unit) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Text(text = server.name, style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = server.url,
                style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Button(onClick = onOpen) { Text(stringResource(R.string.server_open)) }
                Spacer(modifier = Modifier.weight(1f))
                TextButton(onClick = onEdit) {
                    Icon(Icons.Outlined.Edit, contentDescription = null)
                    Text(text = stringResource(R.string.edit), modifier = Modifier.padding(start = 6.dp))
                }
                TextButton(onClick = onRemove) {
                    Icon(Icons.Outlined.Delete, contentDescription = null)
                    Text(text = stringResource(R.string.remove), modifier = Modifier.padding(start = 6.dp))
                }
            }
        }
    }
}

@Composable
private fun ServerDialog(draft: Draft, onDismiss: () -> Unit, onSave: (String, String) -> String?) {
    var name by remember { mutableStateOf(draft.name) }
    var url by remember { mutableStateOf(draft.url) }
    var error by remember { mutableStateOf<String?>(null) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(stringResource(if (draft.id.isEmpty()) R.string.server_add else R.string.server_edit))
        },
        text = {
            Column {
                OutlinedTextField(
                    value = name,
                    onValueChange = { name = it },
                    singleLine = true,
                    label = { Text(stringResource(R.string.server_name)) },
                    placeholder = { Text(stringResource(R.string.server_name_hint)) },
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(modifier = Modifier.height(12.dp))
                OutlinedTextField(
                    value = url,
                    onValueChange = { url = it },
                    singleLine = true,
                    label = { Text(stringResource(R.string.server_url)) },
                    placeholder = { Text(stringResource(R.string.server_url_hint)) },
                    isError = error != null,
                    supportingText = error?.let { { Text(it, color = MaterialTheme.colorScheme.error) } },
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = { TextButton(onClick = { error = onSave(name, url) }) { Text(stringResource(R.string.save)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}
