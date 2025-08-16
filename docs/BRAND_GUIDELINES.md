# TickStorm Brand Guidelines

## Project Identity

### Name
**TickStorm** - High-Performance TCP Stream Server

### Tagline
*"Built for performance, designed for scale."*

### Mission Statement
TickStorm delivers enterprise-grade, high-performance TCP streaming solutions for financial data with sub-millisecond latency and massive concurrency support.

## Visual Identity

### Logo Concept
- **Symbol**: Storm cloud with lightning bolt representing speed and power
- **Typography**: Modern, technical sans-serif font
- **Style**: Minimalist, professional, tech-focused

### Color Palette

#### Primary Colors
- **TickStorm Blue**: `#0066CC` (Primary brand color)
- **Storm Gray**: `#2D3748` (Secondary/text color)
- **Lightning Yellow**: `#F6E05E` (Accent/highlight color)

#### Secondary Colors
- **Success Green**: `#38A169` (Success states, health indicators)
- **Warning Orange**: `#DD6B20` (Warnings, alerts)
- **Error Red**: `#E53E3E` (Errors, critical states)
- **Info Cyan**: `#00B5D8` (Information, links)

#### Neutral Colors
- **White**: `#FFFFFF` (Background, cards)
- **Light Gray**: `#F7FAFC` (Light backgrounds)
- **Medium Gray**: `#A0AEC0` (Borders, dividers)
- **Dark Gray**: `#1A202C` (Dark backgrounds, headers)

### Typography

#### Primary Font
**Inter** - Modern, readable sans-serif
- Headers: Inter Bold (700)
- Body text: Inter Regular (400)
- Code/Technical: Inter Medium (500)

#### Monospace Font
**JetBrains Mono** - For code blocks and technical content
- Code blocks: JetBrains Mono Regular
- Inline code: JetBrains Mono Medium

#### Font Hierarchy
```
H1: Inter Bold, 32px, #2D3748
H2: Inter Bold, 24px, #2D3748
H3: Inter SemiBold, 20px, #2D3748
H4: Inter SemiBold, 18px, #2D3748
Body: Inter Regular, 16px, #2D3748
Small: Inter Regular, 14px, #A0AEC0
Code: JetBrains Mono, 14px, #0066CC
```

## Brand Voice & Tone

### Voice Characteristics
- **Technical**: Precise, accurate, professional
- **Confident**: Authoritative without being arrogant
- **Clear**: Simple, direct communication
- **Performance-focused**: Emphasizing speed, efficiency, scale

### Tone Guidelines
- **Documentation**: Instructional, helpful, comprehensive
- **Marketing**: Confident, performance-focused, professional
- **Technical**: Precise, detailed, accurate
- **Error Messages**: Clear, actionable, non-intimidating

### Language Style
- Use active voice
- Be concise and direct
- Avoid jargon unless necessary
- Include performance metrics when relevant
- Focus on benefits and outcomes

## Brand Applications

### Documentation
- Use consistent header styling with TickStorm Blue
- Include performance badges and metrics
- Use code blocks with proper syntax highlighting
- Include clear navigation and structure

### README Files
- Start with TickStorm logo/title
- Include tagline prominently
- Use consistent badge styling
- Highlight key performance metrics
- Include clear installation/usage sections

### Code Comments
- Use consistent comment styling
- Include performance notes where relevant
- Reference brand standards in configuration

### Error Messages
- Use consistent error formatting
- Include helpful context and solutions
- Maintain professional tone

## Asset Guidelines

### File Naming Conventions
- Use kebab-case for files: `tick-storm-logo.svg`
- Include version numbers: `brand-guidelines-v1.0.md`
- Use descriptive names: `docker-compose-production.yml`

### Directory Structure
```
docs/
├── brand/
│   ├── logos/
│   ├── colors/
│   └── fonts/
├── assets/
│   ├── images/
│   └── icons/
└── templates/
```

### Image Guidelines
- Use SVG for logos and icons
- Optimize PNG/JPG for screenshots
- Maintain consistent aspect ratios
- Include alt text for accessibility

## Usage Examples

### Correct Branding
✅ "TickStorm - High-Performance TCP Stream Server"
✅ "Built for performance, designed for scale"
✅ Using TickStorm Blue (#0066CC) for primary elements
✅ Inter font for headers, JetBrains Mono for code

### Incorrect Branding
❌ "Tick Storm" (separated words)
❌ "tick-storm" (lowercase in titles)
❌ Using random colors not in palette
❌ Mixing different font families inconsistently

## Brand Compliance Checklist

### Documentation
- [ ] TickStorm name used consistently
- [ ] Tagline included where appropriate
- [ ] Color palette followed
- [ ] Typography standards applied
- [ ] File naming conventions followed

### Code
- [ ] Project name consistent in comments
- [ ] Error messages follow tone guidelines
- [ ] Configuration follows naming standards
- [ ] Documentation links use brand colors

### Marketing Materials
- [ ] Logo usage guidelines followed
- [ ] Color palette consistency maintained
- [ ] Typography hierarchy respected
- [ ] Voice and tone guidelines applied

## Implementation Notes

### CSS Variables
```css
:root {
  --tickstorm-blue: #0066CC;
  --storm-gray: #2D3748;
  --lightning-yellow: #F6E05E;
  --success-green: #38A169;
  --warning-orange: #DD6B20;
  --error-red: #E53E3E;
  --info-cyan: #00B5D8;
}
```

### Markdown Badges
```markdown
[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://golang.org/)
[![Performance](https://img.shields.io/badge/Latency-<1ms-0066CC?style=flat)](/)
[![Docker](https://img.shields.io/badge/Docker-Ready-2496ED?style=flat&logo=docker)](/)
```

### Terminal Colors
- Info: Blue (`\033[0;34m`)
- Success: Green (`\033[0;32m`)
- Warning: Yellow (`\033[1;33m`)
- Error: Red (`\033[0;31m`)

## Maintenance

This brand guidelines document should be:
- Reviewed quarterly for consistency
- Updated when new brand elements are added
- Referenced for all new project materials
- Used as training material for contributors

---

**Version**: 1.0  
**Last Updated**: August 16, 2025  
**Next Review**: November 16, 2025
