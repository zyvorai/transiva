import type {ReactNode} from 'react';
import clsx from 'clsx';
import Link from '@docusaurus/Link';
import Layout from '@theme/Layout';
import Heading from '@theme/Heading';
import FeatureHighlights from '@site/src/components/FeatureHighlights';
import Reveal from '@site/src/components/Reveal';

import styles from './index.module.css';

function HomepageHeader() {
  return (
    <header className={clsx('hero hero--primary', styles.heroBanner)}>
      <div className="container">
        <div className={clsx(styles.heroGridSingle, 'text--center')}>
          <Heading as="h1" className="hero__title">
            Escape the renewal trap.
          </Heading>
          <p className="hero__subtitle">
            Transiva is a Go control plane that discovers, inventories, and
            orchestrates workload exports from VMware vSphere and Nutanix
            AHV, handing artifacts to{' '}
            <Link to="https://github.com/zyvorai/h2kvm">h2kvm</Link> for
            conversion and running fleet jobs over REST — Apache-2.0,
            Community Edition.
          </p>
          <div className={styles.buttons}>
            <Link
              className="button button--secondary button--lg"
              to="https://github.com/zyvorai/transiva#60-second-quick-start">
              Get Started
            </Link>
            <Link
              className="button button--outline button--lg button--secondary"
              to="https://github.com/zyvorai/transiva">
              View on GitHub
            </Link>
          </div>
        </div>
      </div>
    </header>
  );
}

function ProblemStatement() {
  return (
    <section className={styles.problem}>
      <div className="container">
        <Reveal className="row">
          <div className="col col--8 col--offset-2 text--center">
            <Heading as="h2" className={styles.sectionHeading}>
              Why Transiva
            </Heading>
            <p>
              Every hypervisor renewal buries your VMs deeper in someone
              else's proprietary API. Getting out usually means a
              spreadsheet, a maintenance window, and a pile of one-off{' '}
              <code>ovftool</code> invocations that fail mid-transfer and
              force you to start over.
            </p>
            <p>
              Transiva turns that into a job: one CLI and job model for both
              vSphere and Nutanix AHV, resumable exports via a REST daemon,
              and offline artifacts — the source VM stays untouched until
              cutover. Export with Transiva, convert with h2kvm, assure with
              GuestKit, operate on Zeus OS.
            </p>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function TrustBand() {
  return (
    <section className={styles.trust}>
      <div className="container">
        <Reveal className={styles.trustGrid}>
          <div>
            <Heading as="h3" className={styles.sectionHeading}>
              Open by default
            </Heading>
            <p>
              Apache-2.0, no phone-home, no agent in the guest. Source,
              Docker, and RPM packaging. Community Edition is free forever
              for labs and single-host PoCs — two source hypervisors, CLI +
              REST, support via GitHub Issues.
            </p>
            <Link to="/docs/ce-vs-enterprise">See CE vs Platform →</Link>
          </div>
          <div className={styles.trustBadges}>
            <img
              src="https://github.com/zyvorai/transiva/actions/workflows/ci.yml/badge.svg"
              alt="CI status"
            />
            <img
              src="https://img.shields.io/badge/License-Apache_2.0-blue.svg"
              alt="Apache 2.0 license"
            />
          </div>
        </Reveal>
      </div>
    </section>
  );
}

function EnterpriseCTA() {
  return (
    <section className={styles.enterprise}>
      <div className="container text--center">
        <Reveal>
          <Heading as="h2" className={styles.sectionHeading}>
            Moving an estate, not a lab?
          </Heading>
          <p className={styles.enterpriseCopy}>
            CE proves export. If you're moving 50–10,000+ VMs, HyperSDK
            Platform adds CBT incremental sync, wave planning, SSO/RBAC, and
            a direct line to Zyvor on cutover night — 10–11 source
            providers instead of 2, and KubeVirt as a deploy target.
          </p>
          <div className={styles.buttons}>
            <Link
              className="button button--primary button--lg"
              to="https://zyvor.dev/poc?utm_source=github-pages&utm_medium=transiva">
              Start a 30-day PoC
            </Link>
            <Link
              className="button button--outline button--lg button--secondary"
              to="https://zyvor.dev/contact?intent=demo&utm_source=github-pages&utm_medium=transiva">
              Book a Platform demo
            </Link>
          </div>
        </Reveal>
      </div>
    </section>
  );
}

export default function Home(): ReactNode {
  return (
    <Layout
      title="Transiva — Enterprise Workload Mobility & Migration Control Plane"
      description="Go control plane that discovers, inventories, and orchestrates workload exports from VMware vSphere and Nutanix AHV. Apache-2.0 Community Edition.">
      <HomepageHeader />
      <main>
        <ProblemStatement />
        <Reveal>
          <FeatureHighlights />
        </Reveal>
        <TrustBand />
        <EnterpriseCTA />
      </main>
    </Layout>
  );
}
