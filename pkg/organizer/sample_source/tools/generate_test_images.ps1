param(
    [Parameter(Mandatory=$true)]
    [int]$Count,
    
    [Parameter(Mandatory=$false)]
    [string]$ExiftoolPath = ".\tools\exiftool.exe",
    
    [Parameter(Mandatory=$false)]
    [string]$SourceDir = ".\",
    
    [Parameter(Mandatory=$false)]
    [string]$OutputDir = ".\"
)

# Check if exiftool exists
if (-not (Test-Path $ExiftoolPath)) {
    Write-Error "Exiftool not found at $ExiftoolPath"
    exit 1
}

# Check if source directory exists
if (-not (Test-Path $SourceDir)) {
    Write-Error "Source directory not found at $SourceDir"
    exit 1
}

# Create output directory if it doesn't exist
if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir
}

# Get all sample images
$sampleImages = Get-ChildItem -Path $SourceDir -Filter "sample*.JPG"
if ($sampleImages.Count -eq 0) {
    Write-Error "No sample images found in $SourceDir"
    exit 1
}

# Generate random images
for ($i = 1; $i -le $Count; $i++) {
    # Pick a random sample image
    $randomSample = $sampleImages | Get-Random
    $outputFile = Join-Path $OutputDir "generated_sample_$($i.ToString('000')).JPG"
    
    # Generate random date components
    $year = Get-Random -Minimum 2020 -Maximum 2024
    $month = Get-Random -Minimum 1 -Maximum 13
    $day = Get-Random -Minimum 1 -Maximum 29
    $hour = Get-Random -Minimum 0 -Maximum 24
    $minute = Get-Random -Minimum 0 -Maximum 60
    $second = Get-Random -Minimum 0 -Maximum 60
    
    $dateStr = "{0:d4}:{1:d2}:{2:d2} {3:d2}:{4:d2}:{5:d2}" -f $year, $month, $day, $hour, $minute, $second
    
    # Copy file and modify EXIF
    Copy-Item $randomSample.FullName $outputFile
    & $ExiftoolPath "-DateTimeOriginal=$dateStr" "-overwrite_original" $outputFile
    
    Write-Host "Generated $outputFile with date $dateStr"
}

Write-Host "Done! Generated $Count test images."