%global debug_package %{nil}

Name:           news-report
Version:        0.5.1
Release:        1%{?dist}
Summary:        Terminal news aggregator for Western authoritative media (EN/DE/FR/ZH)

License:        MIT
URL:            https://github.com/xieguaiwu/news-report
Source0:        %{url}/archive/v%{version}.tar.gz#/%{name}-%{version}.tar.gz

BuildRequires:  golang >= 1.25

%description
news-report is a terminal news aggregator that fetches the latest headlines
from 65+ authoritative Western news sources (Reuters, AP, BBC, Guardian,
NYT, WSJ, Le Monde, FAZ, Die Zeit, Spiegel, DW, IMF, Fed, ECB, Brookings,
CFR and more) across four languages (EN/DE/FR/ZH) and five categories
(US politics / politics / economy / industry / edu-policy).

Features:
- RSS 2.0 / Atom / RDF auto-detection with HTML scrape fallback
- Four-language keyword classification with Chinese (simplified + traditional)
- Interactive TUI with ranking, filtering, and AI reading (optional LLM key)
- Markdown / JSON output for scripting

%prep
%setup -q -n news-report-%{version}

%build
export GOFLAGS="-mod=vendor"
export CGO_ENABLED=0
export GOOS=linux
export GOARCH=amd64

go build -trimpath -ldflags="-s -w" -o news-report .

%install
rm -rf %{buildroot}
install -Dm755 news-report %{buildroot}%{_bindir}/news-report
install -Dm644 LICENSE %{buildroot}%{_defaultlicensedir}/%{name}/LICENSE
install -Dm644 README.md %{buildroot}%{_defaultdocdir}/%{name}/README.md
install -Dm644 README_EN.md %{buildroot}%{_defaultdocdir}/%{name}/README_EN.md

%files
%license LICENSE
%doc README.md README_EN.md
%{_bindir}/news-report

%changelog
* Tue Aug 18 2026 xgw <xieguaiwu@163.com> - 0.5.1-1
- Initial package: four-language news aggregator with TUI (v0.5.1)
