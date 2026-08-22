package com.sunpanel.app

import androidx.compose.foundation.border
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material3.Checkbox
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.width
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
private data class Draft(val server: Server = Server(id = ""), val moi: Boolean = true)

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
    dangNoi: String?,
    loi: String?,
    onOpen: (Server, String) -> Unit,
    onSave: (Server) -> String?,
    onRemove: (String) -> Unit,
) {
    var draft by remember { mutableStateOf<Draft?>(null) }
    var confirmRemove by remember { mutableStateOf<Server?>(null) }
    var hoiMatKhau by remember { mutableStateOf<Server?>(null) }

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
        if (servers.isEmpty() && loi == null) {
            EmptyState(modifier = Modifier.fillMaxSize().padding(padding))
            return@Scaffold
        }

        LazyColumn(
            modifier = Modifier.fillMaxSize().padding(padding),
            contentPadding = PaddingValues(16.dp),
            verticalArrangement = Arrangement.spacedBy(12.dp),
        ) {
            // Lỗi kết nối hiện ngay trên đầu danh sách: nó nói về việc vừa xảy
            // ra, và người dùng đang nhìn vào đúng chỗ đó.
            loi?.let { text ->
                item {
                    Card(
                        modifier = Modifier.fillMaxWidth(),
                        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.errorContainer),
                    ) {
                        Text(
                            text = text,
                            modifier = Modifier.padding(16.dp),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onErrorContainer,
                        )
                    }
                }
            }

            items(servers, key = { it.id }) { server ->
                ServerCard(
                    server = server,
                    dangNoi = dangNoi == server.id,
                    onOpen = {
                        // Máy chủ SSH chưa nhớ mật khẩu thì hỏi ngay tại đây: mật
                        // khẩu root nằm sẵn trên máy là thứ chỉ nên có khi người
                        // dùng tự chọn.
                        if (server.kind == Kind.SSH && server.password.isEmpty()) {
                            hoiMatKhau = server
                        } else {
                            onOpen(server, server.password)
                        }
                    },
                    onEdit = { draft = Draft(server, moi = false) },
                    onRemove = { confirmRemove = server },
                )
            }
        }
    }

    draft?.let { current ->
        ServerDialog(
            draft = current,
            onDismiss = { draft = null },
            onSave = { server ->
                val error = onSave(server)
                if (error == null) draft = null
                error
            },
        )
    }

    hoiMatKhau?.let { server ->
        PasswordDialog(
            server = server,
            onDismiss = { hoiMatKhau = null },
            onConfirm = { matKhau ->
                hoiMatKhau = null
                onOpen(server, matKhau)
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
private fun ServerCard(
    server: Server,
    dangNoi: Boolean,
    onOpen: () -> Unit,
    onEdit: () -> Unit,
    onRemove: () -> Unit,
) {
    Card(
        modifier = Modifier.fillMaxWidth(),
        colors = CardDefaults.cardColors(containerColor = MaterialTheme.colorScheme.surfaceContainer),
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(verticalAlignment = Alignment.CenterVertically) {
                Text(text = server.name, style = MaterialTheme.typography.titleMedium)
                Spacer(modifier = Modifier.width(8.dp))
                KindBadge(server.kind)
            }
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = server.label(),
                style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 2,
                overflow = TextOverflow.Ellipsis,
            )
            Spacer(modifier = Modifier.height(12.dp))
            Row(verticalAlignment = Alignment.CenterVertically) {
                Button(onClick = onOpen, enabled = !dangNoi) {
                    Text(stringResource(if (dangNoi) R.string.connecting else R.string.server_open))
                }
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

/** Huy hiệu cho biết máy chủ này mở bằng cách nào. */
@Composable
private fun KindBadge(kind: Kind) {
    val label = stringResource(if (kind == Kind.SSH) R.string.kind_ssh else R.string.kind_panel)
    val color =
        if (kind == Kind.SSH) MaterialTheme.colorScheme.primary else MaterialTheme.colorScheme.onSurfaceVariant

    Text(
        text = label,
        style = MaterialTheme.typography.labelSmall,
        color = color,
        modifier =
            Modifier.border(1.dp, color, RoundedCornerShape(4.dp)).padding(horizontal = 6.dp, vertical = 1.dp),
    )
}

/** Hỏi mật khẩu ngay trước khi kết nối, cho máy chủ không nhớ sẵn. */
@Composable
private fun PasswordDialog(server: Server, onDismiss: () -> Unit, onConfirm: (String) -> Unit) {
    var matKhau by remember { mutableStateOf("") }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = {
            Text(
                stringResource(
                    if (server.privateKey.isNotBlank()) R.string.ssh_ask_passphrase
                    else R.string.ssh_ask_password
                )
            )
        },
        text = {
            Column {
                Text(
                    server.label(),
                    style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                )
                Spacer(modifier = Modifier.height(12.dp))
                OutlinedTextField(
                    value = matKhau,
                    onValueChange = { matKhau = it },
                    singleLine = true,
                    visualTransformation = PasswordVisualTransformation(),
                    keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                    modifier = Modifier.fillMaxWidth(),
                )
            }
        },
        confirmButton = {
            TextButton(onClick = { onConfirm(matKhau) }) { Text(stringResource(R.string.connect)) }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}

@Composable
private fun ServerDialog(draft: Draft, onDismiss: () -> Unit, onSave: (Server) -> String?) {
    var server by remember { mutableStateOf(draft.server) }
    var nho by remember { mutableStateOf(draft.server.password.isNotEmpty()) }
    var error by remember { mutableStateOf<String?>(null) }

    AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(if (draft.moi) R.string.server_add else R.string.server_edit)) },
        text = {
            Column(modifier = Modifier.verticalScroll(rememberScrollState())) {
                // Hai kiểu kết nối là hai bộ trường khác hẳn nhau, nên chọn kiểu
                // trước rồi mới hiện phần tương ứng.
                SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
                    listOf(Kind.PANEL to R.string.kind_panel, Kind.SSH to R.string.kind_ssh)
                        .forEachIndexed { index, (kind, label) ->
                            SegmentedButton(
                                selected = server.kind == kind,
                                onClick = {
                                    server = server.copy(kind = kind)
                                    error = null
                                },
                                shape = SegmentedButtonDefaults.itemShape(index, 2),
                            ) {
                                Text(stringResource(label))
                            }
                        }
                }
                Spacer(modifier = Modifier.height(12.dp))

                OutlinedTextField(
                    value = server.name,
                    onValueChange = { server = server.copy(name = it) },
                    singleLine = true,
                    label = { Text(stringResource(R.string.server_name)) },
                    placeholder = { Text(stringResource(R.string.server_name_hint)) },
                    modifier = Modifier.fillMaxWidth(),
                )
                Spacer(modifier = Modifier.height(12.dp))

                if (server.kind == Kind.PANEL) {
                    OutlinedTextField(
                        value = server.url,
                        onValueChange = { server = server.copy(url = it) },
                        singleLine = true,
                        label = { Text(stringResource(R.string.server_url)) },
                        placeholder = { Text(stringResource(R.string.server_url_hint)) },
                        isError = error != null,
                        modifier = Modifier.fillMaxWidth(),
                    )
                } else {
                    Row {
                        OutlinedTextField(
                            value = server.host,
                            onValueChange = { server = server.copy(host = it) },
                            singleLine = true,
                            label = { Text(stringResource(R.string.ssh_host)) },
                            placeholder = { Text(stringResource(R.string.ssh_host_hint)) },
                            modifier = Modifier.weight(1f),
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        OutlinedTextField(
                            value = if (server.port == 0) "" else server.port.toString(),
                            onValueChange = { server = server.copy(port = it.toIntOrNull() ?: 0) },
                            singleLine = true,
                            keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                            label = { Text(stringResource(R.string.ssh_port)) },
                            modifier = Modifier.width(92.dp),
                        )
                    }
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = server.user,
                        onValueChange = { server = server.copy(user = it) },
                        singleLine = true,
                        label = { Text(stringResource(R.string.ssh_user)) },
                        placeholder = { Text(stringResource(R.string.ssh_user_hint)) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = server.privateKey,
                        onValueChange = { server = server.copy(privateKey = it) },
                        label = { Text(stringResource(R.string.ssh_key)) },
                        maxLines = 4,
                        textStyle = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    OutlinedTextField(
                        value = server.password,
                        onValueChange = { server = server.copy(password = it) },
                        singleLine = true,
                        visualTransformation = PasswordVisualTransformation(),
                        keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Password),
                        label = { Text(stringResource(R.string.ssh_password)) },
                        modifier = Modifier.fillMaxWidth(),
                    )
                    Row(verticalAlignment = Alignment.CenterVertically) {
                        Checkbox(checked = nho, onCheckedChange = { nho = it })
                        Text(stringResource(R.string.ssh_remember), style = MaterialTheme.typography.bodySmall)
                    }
                    Text(
                        stringResource(R.string.ssh_remember_note),
                        style = MaterialTheme.typography.bodySmall,
                        color = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }

                error?.let {
                    Spacer(modifier = Modifier.height(8.dp))
                    Text(it, color = MaterialTheme.colorScheme.error, style = MaterialTheme.typography.bodySmall)
                }
            }
        },
        confirmButton = {
            TextButton(
                onClick = {
                    // Không đánh dấu nhớ thì mật khẩu chỉ sống trong lần kết nối
                    // này, không đi xuống đĩa.
                    error = onSave(if (nho) server else server.copy(password = ""))
                }
            ) {
                Text(stringResource(R.string.save))
            }
        },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.cancel)) } },
    )
}
