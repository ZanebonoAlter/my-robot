#!/usr/bin/env python3
"""Fix cross-package references in tagmanagement refactoring."""
import os, re, sys

BASE = "/mnt/d/project/Syntopica/backend-go/internal/tagmanagement"
REPO_IMPORT = "syntopica-backend/internal/tagmanagement/repository"
SERVICE_IMPORT = "syntopica-backend/internal/tagmanagement/service"
MODELS_IMPORT = "syntopica-backend/internal/models"
DB_IMPORT = "syntopica-backend/internal/platform/database"

def read_file(path):
    with open(path, 'rb') as f:
        return f.read().decode('utf-8-sig').replace('\r\n', '\n')

def write_file(path, text):
    with open(path, 'wb') as f:
        f.write(text.replace('\n', '\r\n').encode('utf-8'))

def add_import(text, import_path):
    """Add an import path to a Go import block. import_path is WITHOUT quotes."""
    quoted = f'"{import_path}"'
    if quoted in text:
        return text
    
    # Find grouped import block (import ( ... ))
    import_block = re.search(r'import\s*\(\s*\n((?:\s*"[^"]+"\s*\n|\s*\w+\s+"[^"]+"\s*\n|\s*//[^\n]*\n)*)\s*\)', text)
    if import_block:
        prefix = text[:import_block.start()]
        body = import_block.group(1)
        suffix = text[import_block.end():]
        body += f'\t{quoted}\n'
        return prefix + 'import (\n' + body + ')' + suffix
    
    # Find single-line imports
    single = re.findall(r'import\s+"([^"]+)"', text)
    if single:
        all_imports = single + [import_path]
        new_block = 'import (\n'
        for imp in all_imports:
            new_block += f'\t"{imp}"\n'
        new_block += ')'
        result = re.sub(r'import\s+"[^"]+"\s*\n*', '', text)
        pkg = re.search(r'(package \w+\s*\n)', result)
        if pkg:
            return result[:pkg.end()] + '\n' + new_block + '\n' + result[pkg.end():]
        return result
    
    # No import at all - add after package
    pkg = re.search(r'(package \w+\s*\n)', text)
    if pkg:
        return text[:pkg.end()] + f'\nimport "{import_path}"\n\n' + text[pkg.end():]
    
    return text
    
    # No import at all - add after package
    pkg = re.search(r'(package \w+\s*\n)', text)
    if pkg:
        return text[:pkg.end()] + f'\nimport "{import_path}"\n\n' + text[pkg.end():]
    
    return text

def remove_import(text, import_path):
    """Remove an import path from a Go import block. import_path is WITHOUT quotes."""
    quoted = f'"{import_path}"'
    text = text.replace(f'\t{quoted}\n', '')
    text = text.replace(f'{quoted}\n', '')  # for standalone imports
    text = re.sub(r'import\s*\(\s*\n\s*\)', '', text)
    return text

def process_file(filepath, replacements, add_imports, remove_imports_list):
    """Process a single Go file."""
    text = read_file(filepath)
    original = text
    
    # Apply text replacements
    for old, new in replacements:
        text = text.replace(old, new)
    
    # Add needed imports
    for imp in add_imports:
        text = add_import(text, imp)
    
    # Remove unwanted imports
    for imp in remove_imports_list:
        text = remove_import(text, imp)
    
    if text != original:
        write_file(filepath, text)
        return True
    return False

# ============================================================
# SERVICE FILES
# ============================================================

svc_replacements = [
    ('database.DB', 'repository.Repo.DB()'),
    ('InitRepository(database.DB)', 'repository.InitRepository(repository.Repo.DB())'),
    ('repository.Repo.DB().DB()', 'repository.Repo.DB().DB()'),  # no-op, keep as-is
]

svc_add_imports = [REPO_IMPORT]
svc_remove = [DB_IMPORT]

for fname in os.listdir(os.path.join(BASE, 'service')):
    if not fname.endswith('.go'):
        continue
    fpath = os.path.join(BASE, 'service', fname)
    text = read_file(fpath)
    
    # Only add repository import if file actually uses repository.
    after = text.replace('database.DB', 'repository.Repo.DB()')
    needs_repo = 'repository.' in after
    
    add = [REPO_IMPORT] if needs_repo else []
    rem = [DB_IMPORT] if 'database.' not in after and DB_IMPORT in text else []
    
    process_file(fpath, svc_replacements, add, rem)

print("Service files done")

# ============================================================
# HANDLER FILES
# ============================================================

handler_replacements = [
    # DB access
    ('Repo.DB()', 'repository.Repo.DB()'),
    ('database.DB', 'repository.Repo.DB()'),
    ('InitRepository(database.DB)', 'repository.InitRepository(repository.Repo.DB())'),
    
    # Service types/functions
    ('AuxiliaryLabelService', 'service.AuxiliaryLabelService'),
    ('SemanticBoardBackfillService', 'service.SemanticBoardBackfillService'),
    ('NewAuxiliaryLabelService', 'service.NewAuxiliaryLabelService'),
    ('NewSemanticBoardBackfillService', 'service.NewSemanticBoardBackfillService'),
    ('NewSemanticBoardMatchingService', 'service.NewSemanticBoardMatchingService'),
    ('NewSemanticBoardUpgradeService', 'service.NewSemanticBoardUpgradeService'),
    ('SemanticBoardUpgradeDecision', 'service.SemanticBoardUpgradeDecision'),
    ('SemanticBoardMatchConfig', 'service.SemanticBoardMatchConfig'),
    ('MatchDetailResult', 'service.MatchDetailResult'),
    ('boardAuxiliaryLabel', 'service.BoardAuxiliaryLabel'),
    ('matchDetailPair', 'service.MatchDetailPair'),
    ('auxiliaryLabelEmbedder', 'service.AuxiliaryLabelEmbedder'),
    ('defaultAuxiliaryLabelEmbedder', 'service.DefaultAuxiliaryLabelEmbedder'),
    ('auxiliaryLabelEmbeddingModeStorage', 'service.AuxiliaryLabelEmbeddingModeStorage'),
    ('auxiliaryLabelEmbeddingModeMatch', 'service.AuxiliaryLabelEmbeddingModeMatch'),
    ('semanticBoardUpgradeLLM', 'service.SemanticBoardUpgradeLLM'),
    ('newSemanticBoardUpgradeLLM', 'service.NewSemanticBoardUpgradeLLM'),
    
    # Functions
    ('MergeTags(', 'service.MergeTags('),
    ('HardMergeTags(', 'service.HardMergeTags('),
    ('mergeReembeddingQueueFactory()', 'service.EnqueueMergeReembedding'),  # special case
    ('FeedCategoryName(', 'service.FeedCategoryName('),
    ('Slugify(', 'service.Slugify('),
    ('StartFullScan(', 'service.StartFullScan('),
    ('WaitForScanChannel(', 'service.WaitForScanChannel('),
    ('ScanProgress', 'service.ScanProgress'),
    ('StartEvaluation(', 'service.StartEvaluation('),
    ('WaitForEvaluateChannel(', 'service.WaitForEvaluateChannel('),
    ('EvaluateProgress', 'service.EvaluateProgress'),
    ('IsScanRunning(', 'service.IsScanRunning('),
    ('IsEvaluateRunning(', 'service.IsEvaluateRunning('),
    
    # Repository types
    ('NewTagJobQueue(', 'repository.NewTagJobQueue('),
    ('TagJobRequest{', 'repository.TagJobRequest{'),
    ('TagJobQueue', 'repository.TagJobQueue'),
    
    # Queue services
    ('NewEmbeddingQueueService(', 'service.NewEmbeddingQueueService('),
    ('NewMergeReembeddingQueueService(', 'service.NewMergeReembeddingQueueService('),
    ('EmbeddingQueueService', 'service.EmbeddingQueueService'),
    ('MergeReembeddingQueueService', 'service.MergeReembeddingQueueService'),
    
    # Embedding service
    ('NewEmbeddingService(', 'service.NewEmbeddingService('),
    ('EmbeddingService', 'service.EmbeddingService'),
    
    # Config service
    ('NewEmbeddingConfigService(', 'service.NewEmbeddingConfigService('),
    ('EmbeddingConfigService', 'service.EmbeddingConfigService'),
    
    # Tag cache
    ('GetTagCache(', 'service.GetTagCache('),
    ('NewTagCache(', 'service.NewTagCache('),
    
    # Article tagger
    ('TagArticle(', 'service.TagArticle('),
    ('RetagArticle(', 'service.RetagArticle('),
    ('GetArticleTags(', 'service.GetArticleTags('),
    
    # Backfill
    ('BackfillPersonMetadata(', 'service.BackfillPersonMetadata('),
]

handler_add = [SERVICE_IMPORT, REPO_IMPORT]

for fname in os.listdir(os.path.join(BASE, 'handler')):
    if not fname.endswith('.go'):
        continue
    fpath = os.path.join(BASE, 'handler', fname)
    text = read_file(fpath)
    
    # Determine which imports are needed
    after = text
    for old, new in handler_replacements:
        after = after.replace(old, new)
    
    needs_service = 'service.' in after
    needs_repo = 'repository.' in after
    needs_db = 'database.' in after
    
    add = []
    if needs_service:
        add.append(SERVICE_IMPORT)
    if needs_repo:
        add.append(REPO_IMPORT)
    
    rem = []
    if not needs_db and DB_IMPORT in text:
        rem.append(DB_IMPORT)
    
    process_file(fpath, handler_replacements, add, rem)

print("Handler files done")

# ============================================================
# REPOSITORY FILES
# ============================================================

repo_replacements = [
    ('database.DB', 'repository.Repo.DB()'),
    ('InitRepository(database.DB)', 'repository.InitRepository(repository.Repo.DB())'),
]

for fname in os.listdir(os.path.join(BASE, 'repository')):
    if not fname.endswith('.go'):
        continue
    fpath = os.path.join(BASE, 'repository', fname)
    text = read_file(fpath)
    
    after = text
    for old, new in repo_replacements:
        after = after.replace(old, new)
    
    needs_repo = 'repository.' in after
    needs_db = 'database.' in after
    
    add = [REPO_IMPORT] if needs_repo else []
    rem = [DB_IMPORT] if not needs_db and DB_IMPORT in text else []
    
    process_file(fpath, repo_replacements, add, rem)

print("Repository files done")

# ============================================================
# Fix the mergeReembeddingQueueFactory reference in handler
# ============================================================
# The handler file used mergeReembeddingQueueFactory().Enqueue()
# We need to replace it with service.EnqueueMergeReembedding(sourceID, targetID)
# This is a special case because it's not a simple prefix addition

# Let's also fix the import path for models in all files
# (some files use internal/models, others use internal/domain/models)
for root_dir in ['handler', 'service', 'repository']:
    for fname in os.listdir(os.path.join(BASE, root_dir)):
        if not fname.endswith('.go'):
            continue
        fpath = os.path.join(BASE, root_dir, fname)
        text = read_file(fpath)
        if 'internal/domain/models' in text:
            text = text.replace('"syntopica-backend/internal/domain/models"', '"syntopica-backend/internal/models"')
            write_file(fpath, text)

print("Model import paths fixed")

# Create EnqueueMergeReembedding in service/embedding.go if it doesn't exist
emb_path = os.path.join(BASE, 'service', 'embedding.go')
emb_text = read_file(emb_path)
if 'func EnqueueMergeReembedding' not in emb_text:
    # Add after mergeReembeddingQueueFactory
    old = 'var mergeReembeddingQueueFactory = defaultMergeReembeddingQueueFactory'
    new = '''var mergeReembeddingQueueFactory = defaultMergeReembeddingQueueFactory

// EnqueueMergeReembedding enqueues a merge re-embedding task for the given source and target tags.
func EnqueueMergeReembedding(sourceTagID, targetTagID uint) error {
	return mergeReembeddingQueueFactory().Enqueue(sourceTagID, targetTagID)
}'''
    emb_text = emb_text.replace(old, new)
    write_file(emb_path, emb_text)
    print("Added EnqueueMergeReembedding to embedding.go")

# Fix the handler/tag_merge_preview_handler.go special case
# mergeReembeddingQueueFactory().Enqueue(a, b) -> service.EnqueueMergeReembedding(a, b)
hpath = os.path.join(BASE, 'handler', 'tag_merge_preview_handler.go')
htext = read_file(hpath)
htext = re.sub(
    r'mergeReembeddingQueueFactory\(\)\.Enqueue\(([^,]+),\s*([^)]+)\)',
    r'service.EnqueueMergeReembedding(\1, \2)',
    htext
)
write_file(hpath, htext)

print("All done!")
