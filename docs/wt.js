(function () {
  'use strict';

  var form = document.getElementById('header-tool');
  var fileInput = document.getElementById('input-file');
  var options = document.getElementById('tool-options');
  var preview = document.getElementById('preview');
  var suffixTask = document.getElementById('task-suffix');
  var convertTask = document.getElementById('task-convert');
  var suffixOptions = document.getElementById('suffix-options');
  var conversionOptions = document.getElementById('conversion-options');
  var sourceFormat = document.getElementById('source-format');
  var targetFormat = document.getElementById('target-format');
  var customSourceRow = document.getElementById('custom-source-row');
  var customTargetRow = document.getElementById('custom-target-row');
  var suffixInput = document.getElementById('custom-suffix');
  var basicSupported = !!(window.File && window.FileReader && window.Blob &&
    ((window.URL && window.URL.createObjectURL) || window.navigator.msSaveOrOpenBlob));
  var fileSystemAccessAvailable = typeof window.showOpenFilePicker === 'function' &&
    typeof window.showSaveFilePicker === 'function' &&
    typeof window.showDirectoryPicker === 'function' && typeof window.DataTransfer === 'function';
  var selectedFiles = [];
  var selectedHandles = [];
  var previewSourceText = '';
  var selectionVersion = 0;

  if (!basicSupported) {
    window.alert('This browser does not support FASTA Header Tools. Please use a current browser with JavaScript and local file support.');
    fileInput.disabled = true;
    return;
  }

  function listToArray(list) {
    var result = [];
    var index;
    for (index = 0; index < list.length; index += 1) {
      result.push(list[index]);
    }
    return result;
  }

  function errorText(error) {
    return error && error.message ? error.message : 'Unknown error';
  }

  function wasCancelled(error) {
    return error && error.name === 'AbortError';
  }

  function setVisible(element, visible) {
    if (visible) {
      element.removeAttribute('hidden');
      element.style.display = '';
    } else {
      element.setAttribute('hidden', 'hidden');
      element.style.display = 'none';
    }
  }

  function resizePreview() {
    var styles = window.getComputedStyle ? window.getComputedStyle(preview, null) : preview.currentStyle;
    var lineHeight = parseFloat(styles.lineHeight) || 16;
    var lineCount = preview.value.split(/\r\n|\r|\n/).length;
    lineCount = Math.max(20, Math.min(100, lineCount));
    preview.style.height = String(Math.ceil(lineHeight * lineCount + 2)) + 'px';
  }

  function updateTaskVisibility() {
    var suffixMode = suffixTask.checked;
    setVisible(suffixOptions, suffixMode);
    setVisible(conversionOptions, !suffixMode);
  }

  function updateCustomFormatVisibility() {
    setVisible(customSourceRow, sourceFormat.value === 'custom');
    setVisible(customTargetRow, targetFormat.value === 'custom');
  }

  function transformFasta(sourceText) {
    var suffix;
    if (!suffixTask.checked) {
      return null;
    }
    suffix = suffixInput.value;
    return sourceText.replace(/^(\uFEFF?\s*>[^\r\n]*)(?=\r\n|\r|\n|$)/gm, function (header) {
      return header + suffix;
    });
  }

  function readFileText(file, success, failure) {
    var reader = new FileReader();
    reader.onload = function () { success(String(reader.result || '')); };
    reader.onerror = function () { failure(reader.error || new Error('Unable to read the selected file.')); };
    reader.readAsText(file);
  }

  function requireImplementedTask() {
    if (!suffixTask.checked) {
      window.alert('Format conversion is not available yet.');
      return false;
    }
    return true;
  }

  function refreshPreview() {
    if (selectedFiles.length === 0 || !requireImplementedTask()) {
      return;
    }
    preview.value = transformFasta(previewSourceText);
    resizePreview();
  }

  function exportName(name) {
    var lastDot = name.lastIndexOf('.');
    var base = lastDot > 0 ? name.slice(0, lastDot) : name;
    return (base || 'fasta') + '.fasta';
  }

  function uniqueExportName(name, usedNames) {
    var suggestedName = exportName(name);
    var dot;
    var base;
    var extension;
    var index;
    var candidate;
    if (!Object.prototype.hasOwnProperty.call(usedNames, suggestedName)) {
      usedNames[suggestedName] = true;
      return suggestedName;
    }
    dot = suggestedName.lastIndexOf('.');
    base = suggestedName.slice(0, dot);
    extension = suggestedName.slice(dot);
    index = 2;
    candidate = base + ' (' + index + ')' + extension;
    while (Object.prototype.hasOwnProperty.call(usedNames, candidate)) {
      index += 1;
      candidate = base + ' (' + index + ')' + extension;
    }
    usedNames[candidate] = true;
    return candidate;
  }

  function exportItems() {
    var usedNames = {};
    var items = [];
    var index;
    for (index = 0; index < selectedFiles.length; index += 1) {
      items.push({ file: selectedFiles[index], name: uniqueExportName(selectedFiles[index].name, usedNames) });
    }
    return items;
  }

  function outputForFile(file, success, failure) {
    if (file === selectedFiles[0]) {
      success(transformFasta(previewSourceText));
      return;
    }
    readFileText(file, function (sourceText) {
      success(transformFasta(sourceText));
    }, failure);
  }

  function saveDownload(name, output) {
    var blob = new Blob([output], { type: 'text/plain;charset=utf-8' });
    var link;
    var urlApi;
    if (window.navigator.msSaveOrOpenBlob) {
      window.navigator.msSaveOrOpenBlob(blob, name);
      return;
    }
    urlApi = window.URL || window.webkitURL;
    link = document.createElement('a');
    link.href = urlApi.createObjectURL(blob);
    link.download = name;
    link.click();
    window.setTimeout(function () { urlApi.revokeObjectURL(link.href); }, 0);
  }

  function exportDownloads(items, index) {
    if (index >= items.length) {
      return;
    }
    outputForFile(items[index].file, function (output) {
      saveDownload(items[index].name, output);
      window.setTimeout(function () { exportDownloads(items, index + 1); }, 0);
    }, function (error) {
      window.alert('Unable to export the FASTA file: ' + errorText(error));
    });
  }

  function writeFileHandle(handle, file, success, failure) {
    outputForFile(file, function (output) {
      handle.createWritable().then(function (writable) {
        writable.write(output).then(function () {
          writable.close().then(success, failure);
        }, failure);
      }, failure);
    }, failure);
  }

  function writeDirectoryItems(directory, items) {
    var workerCount = Math.min(8, Math.max(2, window.navigator.hardwareConcurrency || 4), items.length);
    var nextIndex = 0;
    var active = 0;
    var completed = 0;
    var failed = false;

    function fail(error) {
      if (!failed) {
        failed = true;
        window.alert('Unable to export the FASTA files: ' + errorText(error));
      }
    }

    function run() {
      var item;
      while (!failed && active < workerCount && nextIndex < items.length) {
        item = items[nextIndex];
        nextIndex += 1;
        active += 1;
        (function (currentItem) {
          directory.getFileHandle(currentItem.name, { create: true }).then(function (handle) {
            writeFileHandle(handle, currentItem.file, function () {
              active -= 1;
              completed += 1;
              if (completed < items.length) {
                run();
              }
            }, fail);
          }, fail);
        }(item));
      }
    }
    run();
  }

  function exportFasta() {
    var items;
    if (selectedFiles.length === 0 || !requireImplementedTask()) {
      return;
    }
    items = exportItems();
    if (fileSystemAccessAvailable && items.length === 1) {
      window.showSaveFilePicker({
        startIn: selectedHandles[0],
        suggestedName: items[0].name,
        types: [{ description: 'FASTA file', accept: { 'text/plain': ['.fasta'] } }]
      }).then(function (handle) {
        writeFileHandle(handle, items[0].file, function () {}, function (error) {
          window.alert('Unable to export the FASTA file: ' + errorText(error));
        });
      }, function (error) {
        if (!wasCancelled(error)) {
          window.alert('Unable to export the FASTA file: ' + errorText(error));
        }
      });
      return;
    }
    if (fileSystemAccessAvailable && items.length > 1) {
      window.showDirectoryPicker({ mode: 'readwrite', startIn: selectedHandles[0] }).then(function (directory) {
        writeDirectoryItems(directory, items);
      }, function (error) {
        if (!wasCancelled(error)) {
          window.alert('Unable to export the FASTA files: ' + errorText(error));
        }
      });
      return;
    }
    exportDownloads(items, 0);
  }

  function resetTool() {
    form.reset();
    selectedFiles = [];
    selectedHandles = [];
    previewSourceText = '';
    selectionVersion += 1;
    options.disabled = true;
    preview.disabled = true;
    preview.value = '';
    updateTaskVisibility();
    updateCustomFormatVisibility();
    resizePreview();
  }

  function useSelectedFiles(files, handles) {
    var currentSelectionVersion;
    if (files.length === 0) {
      resetTool();
      return;
    }
    selectedFiles = files;
    selectedHandles = handles;
    currentSelectionVersion = selectionVersion + 1;
    selectionVersion = currentSelectionVersion;
    readFileText(files[0], function (sourceText) {
      if (selectionVersion !== currentSelectionVersion) {
        return;
      }
      previewSourceText = sourceText;
      options.disabled = false;
      preview.disabled = false;
      preview.value = '';
      resizePreview();
    }, function (error) {
      if (selectionVersion === currentSelectionVersion) {
        resetTool();
        window.alert('Unable to read the selected file: ' + errorText(error));
      }
    });
  }

  function filesFromHandles(handles, success, failure) {
    var files = [];
    var remaining = handles.length;
    var index;
    if (remaining === 0) {
      success(files);
      return;
    }
    for (index = 0; index < handles.length; index += 1) {
      (function (currentIndex) {
        handles[currentIndex].getFile().then(function (file) {
          files[currentIndex] = file;
          remaining -= 1;
          if (remaining === 0) {
            success(files);
          }
        }, failure);
      }(index));
    }
  }

  function openWithFileHandles(event) {
    event.preventDefault();
    window.showOpenFilePicker({ multiple: true }).then(function (handles) {
      filesFromHandles(handles, function (files) {
        var transfer = new DataTransfer();
        var index;
        for (index = 0; index < files.length; index += 1) {
          transfer.items.add(files[index]);
        }
        try {
          fileInput.files = transfer.files;
        } catch (ignore) {}
        useSelectedFiles(files, handles);
      }, function (error) {
        window.alert('Unable to open the selected files: ' + errorText(error));
      });
    }, function (error) {
      if (!wasCancelled(error)) {
        window.alert('Unable to open the selected files: ' + errorText(error));
      }
    });
  }

  form.addEventListener('submit', function (event) { event.preventDefault(); });
  if (fileSystemAccessAvailable) {
    fileInput.addEventListener('click', openWithFileHandles);
  } else {
    fileInput.addEventListener('change', function () { useSelectedFiles(listToArray(fileInput.files), []); });
  }
  suffixTask.addEventListener('change', updateTaskVisibility);
  convertTask.addEventListener('change', updateTaskVisibility);
  sourceFormat.addEventListener('change', updateCustomFormatVisibility);
  targetFormat.addEventListener('change', updateCustomFormatVisibility);
  document.getElementById('reset-suffix').addEventListener('click', function () { suffixInput.value = ''; });
  document.getElementById('refresh-preview').addEventListener('click', refreshPreview);
  document.getElementById('export-file').addEventListener('click', exportFasta);
  document.getElementById('reset-tool').addEventListener('click', resetTool);
  updateTaskVisibility();
  updateCustomFormatVisibility();
  resizePreview();
}());
